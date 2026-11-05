import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { LoadingState, UserAvatar } from '../components/ui'
import type { GameDetail, UnoSnapshot } from '../types'
import './uno-game.css'

const UNO_GAME_ASSET_PATH = '/files/uno/index.html'
const UNO_GAME_ASSET_VERSION = 'radial-table-20261105-v21'
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

export function UnoGamePage() {
  const [searchParams] = useSearchParams()
  const { user, loading: authLoading } = useAuth()
  const frameRef = useRef<HTMLIFrameElement>(null)
  const gameID = searchParams.get('game')?.trim() ?? ''
  const spectatorMode = searchParams.get('spectate') === '1'
  const [game, setGame] = useState<GameDetail | null>(null)
  const [unoState, setUnoState] = useState<UnoSnapshot | null>(null)
  const [loading, setLoading] = useState(Boolean(gameID))
  const [error, setError] = useState('')
  const [accessGranted, setAccessGranted] = useState(!gameID)

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
    let active = true
    setLoading(true)
    setError('')
    setAccessGranted(false)
    const role = spectatorMode ? 'spectator' : 'player'
    const enter = async () => {
      try {
        if (spectatorMode) await api.spectateGame(gameID)
        else await api.reconnectGame(gameID)
        const value = await api.getGame(gameID)
        let snapshot: UnoSnapshot | null = null
        try { snapshot = await api.getUnoState(gameID) } catch { /* The iframe retries while the game state is starting. */ }
        if (active) {
          setGame(value)
          setUnoState(snapshot)
          setAccessGranted(true)
        }
      } catch {
        if (active) setError(spectatorMode ? 'This public game cannot be spectated.' : 'Only the player who was already in this game can reconnect.')
      } finally {
        if (active) setLoading(false)
      }
    }
    void enter()
    const heartbeat = window.setInterval(() => { void api.updateGamePresence(gameID, role).catch(() => undefined) }, 10000)
    return () => { active = false; window.clearInterval(heartbeat) }
  }, [authLoading, gameID, spectatorMode, user])

  useEffect(() => {
    if (!gameID || !accessGranted) return
    let active = true
    const refresh = async () => {
      try {
        const value = await api.getGame(gameID, true)
        let snapshot: UnoSnapshot | null = null
        try { snapshot = await api.getUnoState(gameID) } catch { /* Keep the last snapshot during a short API interruption. */ }
        if (active) {
          setGame(value)
          if (snapshot) setUnoState(snapshot)
        }
      } catch {
        // The 3D scene remains available if the lobby context is briefly unavailable.
      }
    }
    const interval = window.setInterval(() => void refresh(), 3000)
    return () => {
      active = false
      window.clearInterval(interval)
    }
  }, [accessGranted, gameID])

  const currentGame = game?.game
  const participants = game?.participants ?? []
  const spectators = game?.spectators ?? []
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

  const sendBootstrap = () => {
    frameRef.current?.contentWindow?.postMessage({
      type: 'uno.bootstrap',
      gameId: gameID || null,
      player: user ? { id: user.id, username: user.username, avatarUrl: user.avatarUrl } : null,
      game,
    }, window.location.origin)
  }

  return <main className="uno-game-page">
    {error ? <p className="uno-game-warning">{error}</p> : null}
    {loading ? <div className="uno-game-loading"><LoadingState label="Loading match" /></div> : null}
    {accessGranted ? <>
        <iframe ref={frameRef} className="uno-game-frame" title="Uno 3D game" src={frameSrc} onLoad={sendBootstrap} allow="fullscreen; pointer-lock" />
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
          <div className="uno-game-info-footer"><span>Round</span><strong>{unoState?.turnNumber ?? '—'}</strong><span className="uno-game-muted">{unoState?.turnPlayerId ? `Turn: ${tablePlayers.find((player) => player.userId === unoState.turnPlayerId)?.username ?? 'player'}` : 'Game state is managed by the table'}</span></div>
        </aside>

        <div className="uno-game-spectator-bar"><span className="uno-game-overlay-eyebrow">Spectators</span>{spectators.length > 0 ? <strong>{spectators.map((spectator) => spectator.username).join(', ')}</strong> : <span className="uno-game-muted">None yet</span>}</div>
      </section>
    </> : <div className="uno-game-unavailable"><strong>{loading ? 'Joining game…' : 'Game access denied'}</strong><p>{loading ? 'Restoring your session.' : error}</p><Link className="button button-secondary" to="/games">Back to tables</Link></div>}
  </main>
}
