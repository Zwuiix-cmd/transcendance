import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ApiClientError, api } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { LoadingState, UserAvatar } from '../components/ui'
import { UnoLoadingScreen } from '../components/UnoLoadingScreen'
import { useRealtimeResync, useRealtimeTopic } from '../realtime/hooks'
import type { GameDeletedEvent, GameDetail, UnoActionRequest, UnoSnapshot } from '../types'
import { requestErrorMessage } from '../utils/errors'
import './uno-game.css'

const UNO_GAME_ASSET_PATH = '/files/uno/index.html'
const UNO_GAME_ASSET_VERSION = 'rules-20261114-v30'
const BOT_NAMES = ['Nova', 'Pixel', 'Orbit', 'Mochi', 'Rook', 'Echo', 'Ziggy', 'Comet', 'Miso', 'Vega', 'Tango', 'Kairo']
const BOT_COLORS = ['#a78bfa', '#f0abfc', '#c4b5fd', '#93c5fd', '#f9a8d4', '#86efac', '#fcd34d', '#fdba74']

function botAvatarUrl(matchID: string, botID: string) {
  const seed = `${matchID}:${botID}`
  const hash = [...seed].reduce((value, character) => ((value * 31) + character.charCodeAt(0)) >>> 0, 2166136261)
  const color = BOT_COLORS[hash % BOT_COLORS.length]
  const accent = BOT_COLORS[(hash >>> 8) % BOT_COLORS.length]
  const eyeY = 30 + (hash % 8)
  const mouthY = 47 + ((hash >>> 4) % 7)
  const shape = hash % 3
  const decoration = shape === 0
    ? `<path d="M10 17L24 7L31 18" fill="${accent}" opacity=".85"/><path d="M70 17L56 7L49 18" fill="${accent}" opacity=".85"/>`
    : shape === 1
      ? `<circle cx="13" cy="14" r="7" fill="${accent}" opacity=".85"/><circle cx="67" cy="14" r="7" fill="${accent}" opacity=".85"/>`
      : `<rect x="9" y="9" width="13" height="13" rx="3" fill="${accent}" opacity=".85"/><rect x="58" y="9" width="13" height="13" rx="3" fill="${accent}" opacity=".85"/>`
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 80 80"><rect width="80" height="80" fill="${color}"/>${decoration}<circle cx="40" cy="40" r="24" fill="#17131f" opacity=".92"/><circle cx="32" cy="${eyeY}" r="4" fill="${accent}"/><circle cx="48" cy="${eyeY}" r="4" fill="${accent}"/><path d="M28 ${mouthY}c7 8 17 8 24 0" fill="none" stroke="${accent}" stroke-width="4" stroke-linecap="square"/></svg>`
  return `data:image/svg+xml,${encodeURIComponent(svg)}`
}

function botNameOrder(matchID: string) {
  return [...BOT_NAMES].sort((first, second) => {
    const firstScore = [...`${matchID}:${first}`].reduce((score, character) => (score * 31 + character.charCodeAt(0)) >>> 0, 7)
    const secondScore = [...`${matchID}:${second}`].reduce((score, character) => (score * 31 + character.charCodeAt(0)) >>> 0, 7)
    return firstScore - secondScore
  })
}

type FrameMessage =
  | { type: 'uno.ready' }
  | { type: 'uno.progress'; stage: string; ratio: number }
  | { type: 'uno.scene.ready' }
  | { type: 'uno.animation.finished'; version: number }
  | { type: 'uno.action'; id: number; action: UnoActionRequest }

function newerSnapshot(current: UnoSnapshot | null, next: UnoSnapshot) {
  return current && current.version >= next.version ? current : next
}

async function fetchSnapshot(gameID: string) {
  for (let attempt = 0; attempt < 3; attempt += 1) {
    try {
      return await api.getUnoState(gameID)
    } catch (error) {
      if (error instanceof ApiClientError && error.status < 500 && error.status !== 409) return null
      await new Promise((resolve) => window.setTimeout(resolve, 400 * (attempt + 1)))
    }
  }
  return null
}

export function UnoGamePage() {
  const [searchParams] = useSearchParams()
  const { user, loading: authLoading } = useAuth()
  const frameRef = useRef<HTMLIFrameElement>(null)
  const gameID = searchParams.get('game')?.trim() ?? ''
  const spectatorMode = searchParams.get('spectate') === '1'
  const [game, setGame] = useState<GameDetail | null>(null)
  const [unoState, setUnoState] = useState<UnoSnapshot | null>(null)
  const [sceneProgress, setSceneProgress] = useState({ stage: 'joining', ratio: 0 })
  const [sceneReady, setSceneReady] = useState(false)
  const [loaderGone, setLoaderGone] = useState(false)
  const [loaderStalled, setLoaderStalled] = useState(false)
  useEffect(() => {
    if (!sceneReady) return undefined
    const timer = window.setTimeout(() => setLoaderGone(true), 800)
    return () => window.clearTimeout(timer)
  }, [sceneReady])
  useEffect(() => {
    if (sceneReady || loaderGone) return undefined
    setLoaderStalled(false)
    const timer = window.setTimeout(() => setLoaderStalled(true), 20000)
    return () => window.clearTimeout(timer)
  }, [loaderGone, sceneProgress.stage, sceneReady])
  const [loading, setLoading] = useState(Boolean(gameID))
  const [error, setError] = useState('')
  const [accessGranted, setAccessGranted] = useState(!gameID)
  const latest = useRef({ game, unoState, user })
  latest.current = { game, unoState, user }

  const applySnapshot = useCallback((snapshot: UnoSnapshot) => setUnoState((current) => newerSnapshot(current, snapshot)), [])

  const enter = useCallback(async (initial: boolean) => {
    if (!gameID || !user) return
    if (initial) {
      setLoading(true)
      setError('')
      setAccessGranted(false)
    }
    try {
      if (spectatorMode) await api.spectateGame(gameID)
      else await api.reconnectGame(gameID)
      const detail = await api.getGame(gameID)
      const snapshot = await fetchSnapshot(gameID)
      setGame(detail)
      if (snapshot) applySnapshot(snapshot)
      setAccessGranted(true)
    } catch (requestError) {
      if (!initial) return
      const code = requestError instanceof ApiClientError ? requestError.code : undefined
      setError(code === 'game_finished' ? 'This game has already finished.' : spectatorMode ? 'This public game cannot be spectated.' : requestErrorMessage(requestError, 'Only the player who was already in this game can reconnect.'))
    } finally {
      if (initial) setLoading(false)
    }
  }, [applySnapshot, gameID, spectatorMode, user])

  useEffect(() => {
    if (authLoading) {
      setLoading(Boolean(gameID))
      return
    }
    if (!gameID || !user) {
      setLoading(false)
      setAccessGranted(!gameID)
      if (gameID) setError('Sign in to enter this game.')
      return
    }
    void enter(true)
  }, [authLoading, enter, gameID, user])

  useRealtimeTopic<GameDetail | UnoSnapshot | GameDeletedEvent>(gameID && accessGranted && user ? `game:${gameID}` : null, (event) => {
    if (event.event === 'game.updated') setGame(event.payload as GameDetail)
    if (event.event === 'uno.state') applySnapshot(event.payload as UnoSnapshot)
    if (event.event === 'game.deleted') {
      setAccessGranted(false)
      setError('This table was closed by its host.')
    }
  })
  useRealtimeResync(() => { if (accessGranted) void enter(false) })

  const postToFrame = useCallback((message: Record<string, unknown>) => {
    frameRef.current?.contentWindow?.postMessage(message, window.location.origin)
  }, [])

  const runAction = useCallback(async (id: number, action: UnoActionRequest) => {
    try {
      const snapshot = await api.unoAction(gameID, action)
      applySnapshot(snapshot)
      postToFrame({ type: 'uno.action.result', id, ok: true })
    } catch (requestError) {
      postToFrame({ type: 'uno.action.result', id, ok: false, message: requestErrorMessage(requestError, 'The table refused this move.') })
    }
  }, [applySnapshot, gameID, postToFrame])

  const [panelBusy, setPanelBusy] = useState(false)
  const [panelError, setPanelError] = useState('')
  const panelAction = useCallback(async (action: UnoActionRequest) => {
    if (panelBusy) return
    setPanelBusy(true)
    setPanelError('')
    try {
      applySnapshot(await api.unoAction(gameID, action))
    } catch (requestError) {
      setPanelError(requestErrorMessage(requestError, 'The table refused this move.'))
    } finally {
      setPanelBusy(false)
    }
  }, [applySnapshot, gameID, panelBusy])

  useEffect(() => {
    if (!accessGranted || !gameID) return undefined
    const onMessage = (event: MessageEvent<FrameMessage>) => {
      if (event.origin !== window.location.origin || !frameRef.current || event.source !== frameRef.current.contentWindow) return
      const data = event.data
      if (!data || typeof data !== 'object') return
      if (data.type === 'uno.ready') {
        const { game: currentGame, unoState: snapshot, user: viewer } = latest.current
        postToFrame({ type: 'uno.bootstrap', gameId: gameID, spectator: spectatorMode, player: viewer ? { id: viewer.id, username: viewer.username, avatarUrl: viewer.avatarUrl } : null, game: currentGame, snapshot })
      }
      if (data.type === 'uno.progress' && typeof data.stage === 'string') setSceneProgress({ stage: data.stage, ratio: typeof data.ratio === 'number' ? data.ratio : 0 })
      if (data.type === 'uno.scene.ready') setSceneReady(true)
      if (data.type === 'uno.animation.finished' && latest.current.unoState?.status === 'finished' && data.version === latest.current.unoState.version) setShowResults(true)
      if (data.type === 'uno.action' && typeof data.id === 'number' && data.action && !spectatorMode) void runAction(data.id, data.action)
    }
    window.addEventListener('message', onMessage)
    return () => window.removeEventListener('message', onMessage)
  }, [accessGranted, gameID, postToFrame, runAction, spectatorMode])

  useEffect(() => {
    if (unoState) postToFrame({ type: 'uno.state', snapshot: unoState })
  }, [postToFrame, unoState])

  const currentGame = game?.game
  const participants = game?.participants ?? []
  const spectators = game?.spectators ?? []
  const [showResults, setShowResults] = useState(false)
  useEffect(() => {
    if (unoState?.status !== 'finished') {
      setShowResults(false)
    }
    return undefined
  }, [unoState?.status])
  const results = useMemo(() => {
    if (!unoState || unoState.status !== 'finished' || !unoState.results?.length) return null
    return unoState.results.map((result) => ({
      ...result,
      displayName: result.playerId === user?.id ? `${result.name} (you)` : result.name,
      points: result.winner ? result.pointsEarned : -result.handPoints,
    }))
  }, [unoState, user?.id])
  const rules = unoState?.rules ?? currentGame?.settings ?? null
  const houseRules = useMemo(() => {
    if (!rules) return []
    const active: string[] = []
    if (rules.drawPenalty !== 2) active.push(`+${rules.drawPenalty} draw penalty`)
    if (rules.stackDrawCards) active.push('Stacking')
    if (rules.jumpIn) active.push('Jump-in')
    if (rules.sevenZero) active.push('Seven-zero')
    if (rules.drawUntilPlayable) active.push('Draw until playable')
    if (rules.forcePlay) active.push('Force play')
    if (!rules.challengeDrawFour) active.push('No +4 challenge')
    return active
  }, [rules])
  const myPending = Boolean(!spectatorMode && user && unoState?.status === 'active' && unoState.turnPlayerId === user.id && (unoState.phase === 'challenge' || unoState.phase === 'penalty'))
  const mySwap = Boolean(!spectatorMode && user && unoState?.status === 'active' && unoState.turnPlayerId === user.id && unoState.phase === 'swap')
  const seatCount = Math.min(8, currentGame?.settings.botsEnabled ? currentGame.settings.maxPlayers : Math.max(participants.length, 1))
  const handSize = currentGame?.settings.startingHandSize ?? 7
  const orderedBotNames = useMemo(() => botNameOrder(gameID), [gameID])
  const tablePlayers = useMemo(() => {
    if (unoState?.players?.length) {
      return unoState.players.map((player) => ({
        userId: player.id,
        username: player.name,
        host: player.id === currentGame?.createdBy,
        avatarUrl: player.bot ? botAvatarUrl(gameID, player.id) : player.id === user?.id ? user.avatarUrl : null,
        bot: player.bot,
        cardCount: player.cardCount,
        finished: player.finished,
      }))
    }
    const players = participants.map((participant) => ({
      userId: participant.userId,
      username: participant.username,
      host: participant.host,
      avatarUrl: participant.userId === user?.id ? user.avatarUrl : null,
      bot: false,
      cardCount: 0,
      finished: false,
    }))
    if (!currentGame?.settings.botsEnabled) return players
    for (let index = players.length; index < seatCount; index += 1) {
      players.push({
        userId: `bot-${gameID}-${index}`,
        username: orderedBotNames[index % orderedBotNames.length],
        host: false,
        avatarUrl: botAvatarUrl(gameID, `bot-${gameID}-${index}`),
        bot: true,
        cardCount: 0,
        finished: false,
      })
    }
    return players
  }, [currentGame?.createdBy, currentGame?.settings.botsEnabled, gameID, orderedBotNames, participants, seatCount, unoState, user])
  const turnLabel = useMemo(() => {
    if (!unoState) return 'Game state is managed by the table'
    const nameOf = (id?: string) => tablePlayers.find((player) => player.userId === id)?.username ?? 'player'
    if (unoState.status === 'finished') return `Winner: ${nameOf(unoState.winnerId)} (+${unoState.results?.[0]?.pointsEarned ?? 0} pts)`
    if (unoState.phase === 'challenge' || unoState.phase === 'penalty') return `${nameOf(unoState.turnPlayerId)} faces a +${unoState.pendingDraw ?? 4} from ${nameOf(unoState.pendingFromId)}`
    if (unoState.phase === 'swap') return `${nameOf(unoState.turnPlayerId)} picks a player to swap hands with`
    if (unoState.phase === 'drawn') return `${nameOf(unoState.turnPlayerId)} drew a playable card`
    return `Turn: ${nameOf(unoState.turnPlayerId)}${unoState.direction < 0 ? ' (reversed)' : ''}`
  }, [tablePlayers, unoState])

  const frameSrc = useMemo(() => {
    const query = new URLSearchParams()
    if (gameID) query.set('gameId', gameID)
    if (user?.id) query.set('playerId', user.id)
    if (spectatorMode) query.set('spectate', '1')
    query.set('assets', UNO_GAME_ASSET_VERSION)
    query.set('seats', String(seatCount))
    query.set('hand', String(handSize))
    return `${UNO_GAME_ASSET_PATH}${query.size ? `?${query.toString()}` : ''}`
  }, [gameID, handSize, seatCount, spectatorMode, user?.id])

  return <main className="uno-game-page">
    {error ? <p className="uno-game-warning">{error}</p> : null}
    {loading && loaderGone ? <div className="uno-game-loading"><LoadingState label="Loading match" /></div> : null}
    {!loaderGone && (loading || accessGranted) ? <UnoLoadingScreen stage={accessGranted ? sceneProgress.stage : 'joining'} ratio={accessGranted ? Math.max(sceneProgress.ratio, 0.05) : 0.02} seats={tablePlayers} difficulty={currentGame?.settings.botDifficulty} spectator={spectatorMode} leaving={sceneReady} stalled={loaderStalled} onSkip={() => setLoaderGone(true)} /> : null}
    {accessGranted ? <>
        <iframe ref={frameRef} className="uno-game-frame" title="Uno 3D game" src={frameSrc} allow="fullscreen" />
      {myPending && unoState ? <section className="uno-action-panel" aria-label="Draw penalty">
        <div className="uno-action-copy">
          <span className="uno-game-overlay-eyebrow">{unoState.phase === 'challenge' ? 'Wild Draw Four' : 'Draw penalty'}</span>
          <strong>{tablePlayers.find((player) => player.userId === unoState.pendingFromId)?.username ?? 'A player'} hit you with a +{unoState.pendingDraw ?? 4}</strong>
          <span>{unoState.phase === 'challenge' ? 'Draw the cards, or challenge if you think they still held the previous color.' : 'Draw the cards to end your turn.'}{unoState.rules?.stackDrawCards ? ' You can also stack a matching draw card from your hand.' : ''}</span>
          {panelError ? <span className="uno-action-error" role="alert">{panelError}</span> : null}
        </div>
        <div className="uno-action-buttons">
          <button className="uno-action-button is-secondary" type="button" onClick={() => void panelAction({ type: 'accept' })} disabled={panelBusy}>Draw {unoState.pendingDraw ?? 4}</button>
          {unoState.phase === 'challenge' ? <button className="uno-action-button" type="button" onClick={() => void panelAction({ type: 'challenge' })} disabled={panelBusy}>{panelBusy ? 'Challenging…' : 'Challenge +4'}</button> : null}
        </div>
      </section> : null}
      {mySwap ? <section className="uno-action-panel" aria-label="Hand swap">
        <div className="uno-action-copy">
          <span className="uno-game-overlay-eyebrow">Seven-zero</span>
          <strong>You played a 7: pick who you swap hands with</strong>
          {panelError ? <span className="uno-action-error" role="alert">{panelError}</span> : null}
        </div>
        <div className="uno-action-buttons">{tablePlayers.filter((player) => player.userId !== user?.id && !player.finished).map((player) => <button key={player.userId} className="uno-action-button is-secondary" type="button" onClick={() => void panelAction({ type: 'swap', targetId: player.userId })} disabled={panelBusy}>{player.username} · {player.cardCount}</button>)}</div>
      </section> : null}

      <section className="uno-game-overlay" aria-label="Match information">
        <div className="uno-game-overlay-header">
          <div className="uno-game-header-meta">{gameID ? <span>#{gameID.slice(0, 8)}</span> : null}</div>
        </div>

        <aside className="uno-game-info-card">
          <div className="uno-game-info-title"><div><span className="uno-game-overlay-eyebrow">Match overview</span><strong>{spectatorMode ? 'Watching the table' : 'You are in the table'}</strong></div><span className="uno-game-info-icon">✦</span></div>
          <div className="uno-game-info-stats">
            <div><span>Players</span><strong>{tablePlayers.length}</strong></div>
            <div><span>Seats</span><strong>{seatCount}</strong></div>
            <div><span>Spectators</span><strong>{currentGame?.spectatorCount ?? spectators.length}</strong></div>
            <div><span>Turn limit</span><strong>{currentGame ? `${currentGame.turnTimeLimit}s` : '—'}</strong></div>
          </div>
          <div className="uno-game-roster"><span className="uno-game-overlay-eyebrow">At the table</span><div className="uno-game-roster-list">{tablePlayers.length > 0 ? tablePlayers.map((player) => <span className="uno-game-roster-player" key={player.userId}><UserAvatar username={player.username} avatarUrl={player.avatarUrl} className="uno-game-roster-avatar" />{player.username}{player.bot ? <small>Bot</small> : player.host ? <small>Host</small> : null}</span>) : <span className="uno-game-muted">Waiting for players</span>}</div></div>
          <div className="uno-game-info-footer"><span>Round</span><strong>{unoState?.turnNumber ?? '—'}</strong><span className="uno-game-muted">{turnLabel}</span></div>
          <div className="uno-game-rules"><span className="uno-game-overlay-eyebrow">{houseRules.length ? 'House rules on this table' : 'Official rules'}</span>{houseRules.length ? <div className="uno-game-rules-pills">{houseRules.map((rule) => <span key={rule}>{rule}</span>)}</div> : null}<ul><li>Match the color, the number or the symbol, or play a Wild.</li><li>Draw one card if you cannot play. If it fits, play it, or click the pile again to pass.</li><li>+2 and Skip hit the next player. Reverse flips the order, and acts as a Skip with two players.</li><li>Wild +4: the target presses Enter to draw, or C to challenge a bluff.</li><li>Press U to call UNO on your second-to-last card, or click a forgetful player's hand to catch them (+2).</li></ul></div>
        </aside>

        {showResults && results ? <div className="uno-game-results" role="dialog" aria-label="Round results">
          <span className="uno-game-overlay-eyebrow">Round over</span>
          <h2>{results[0].playerId === user?.id ? 'You won the round!' : `${results[0].name} wins the round`}</h2>
          <ol className="uno-game-results-list">{results.map((player) => <li key={player.playerId} className={player.winner ? 'is-winner' : undefined}><span className="uno-game-results-rank">{player.rank}</span><span className="uno-game-results-name">{player.displayName}{player.bot ? <small>Bot</small> : null}</span><span className="uno-game-results-cards">{player.winner ? 'UNO out' : `${player.cardsRemaining} card${player.cardsRemaining === 1 ? '' : 's'} left`}</span><strong className="uno-game-results-points">{player.points > 0 ? `+${player.points}` : player.points} pts</strong></li>)}</ol>
          <div className="uno-game-results-actions"><Link className="button" to="/games">Back to tables</Link><Link className="button button-secondary" to="/dashboard">Home</Link></div>
        </div> : null}
        {unoState?.status === 'finished' && !showResults ? <div className="uno-game-results-shortcut"><Link className="button button-secondary" to="/games">Back to tables</Link></div> : null}

        <div className="uno-game-spectator-bar"><span className="uno-game-overlay-eyebrow">Spectators</span>{spectators.length > 0 ? <strong>{spectators.map((spectator) => spectator.username).join(', ')}</strong> : <span className="uno-game-muted">None yet</span>}</div>
      </section>
    </> : <div className="uno-game-unavailable"><strong>{loading ? 'Joining game…' : 'Game access denied'}</strong><p>{loading ? 'Restoring your session.' : error}</p><Link className="button button-secondary" to="/games">Back to tables</Link></div>}
  </main>
}
