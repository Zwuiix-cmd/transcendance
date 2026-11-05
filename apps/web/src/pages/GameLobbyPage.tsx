import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api } from '../api/client'
import { Badge, Button, Card, LoadingState, PageHeader, UserAvatar } from '../components/ui'
import { DEFAULT_GAME_SETTINGS } from '../gameSettings'
import type { GameDetail } from '../types'
import { requestErrorMessage } from '../utils/errors'
import './game-lobby.css'

function statusLabel(status: GameDetail['game']['status']) {
  return status === 'in_progress' ? 'In progress' : status[0].toUpperCase() + status.slice(1)
}

function statusTone(status: GameDetail['game']['status']) {
  return status === 'waiting' ? 'warning' : status === 'finished' ? 'neutral' : 'success'
}

export function GameLobbyPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [detail, setDetail] = useState<GameDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState('')

  const load = async (showLoading = true) => {
    if (!id) return
    if (showLoading) setLoading(true)
    setError('')
    try {
      const nextDetail = await api.getGame(id, !showLoading)
      if (nextDetail.game.status === 'in_progress') {
        if (nextDetail.game.joined) {
          navigate(`/uno?game=${encodeURIComponent(nextDetail.game.id)}`)
          return
        }
        if (nextDetail.game.spectating) {
          navigate(`/uno?game=${encodeURIComponent(nextDetail.game.id)}&spectate=1`)
          return
        }
      }
      if (nextDetail.game.joined) void api.updateGamePresence(id, 'player')
      if (nextDetail.game.spectating) void api.updateGamePresence(id, 'spectator')
      setDetail(nextDetail)
    } catch (requestError) {
      setError(requestErrorMessage(requestError, 'Could not load this game lobby.'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
    const heartbeat = window.setInterval(() => {
      void load(false)
    }, 3000)
    return () => window.clearInterval(heartbeat)
  }, [id])

  const copyCode = async () => {
    const code = detail?.game.joinCode
    if (!code) return
    try {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1800)
    } catch {
      setError('Could not copy the invite code.')
    }
  }

  const startGame = async () => {
    if (!detail) return
    setBusy(true); setError('')
    try {
      await api.startUnoGame(detail.game.id)
      navigate(`/uno?game=${encodeURIComponent(detail.game.id)}`)
    } catch (requestError) {
      setError(requestErrorMessage(requestError, 'Could not start the game.'))
    } finally {
      setBusy(false)
    }
  }

  const spectate = async () => {
    if (!detail) return
    setBusy(true); setError('')
    try {
      await api.spectateGame(detail.game.id)
      navigate(`/uno?game=${encodeURIComponent(detail.game.id)}&spectate=1`)
    } catch (requestError) {
      setError(requestErrorMessage(requestError, 'Could not join spectator mode.'))
    } finally {
      setBusy(false)
    }
  }

  const leave = async () => {
    if (!detail) return
    setBusy(true)
    setError('')
    try {
      await api.leaveGame(detail.game.id)
      navigate('/games')
    } catch (requestError) {
      setError(requestErrorMessage(requestError, 'Could not leave the lobby.'))
    } finally {
      setBusy(false)
    }
  }

  const rules = useMemo(() => {
    const settings = detail?.game.settings ?? DEFAULT_GAME_SETTINGS
    return [
      ['Turn limit', `${detail?.game.turnTimeLimit ?? 15}s`],
      ['Players', `${detail?.game.participantCount ?? 0}/${settings.maxPlayers}`],
      ['Starting hand', `${settings.startingHandSize} cards`],
      ['Draw penalty', `+${settings.drawPenalty}`],
      ['Stack draw cards', settings.stackDrawCards ? 'On' : 'Off'],
      ['Jump-in', settings.jumpIn ? 'On' : 'Off'],
      ['Seven–zero', settings.sevenZero ? 'On' : 'Off'],
      ['Draw until playable', settings.drawUntilPlayable ? 'On' : 'Off'],
      ['Force play', settings.forcePlay ? 'On' : 'Off'],
      ['Challenge +4', settings.challengeDrawFour ? 'On' : 'Off'],
      ['Multiple decks', settings.allowMultipleDecks ? 'On' : 'Off'],
      ['Bots', settings.botsEnabled ? 'Enabled' : 'Disabled'],
    ]
  }, [detail])

  if (loading) return <main className="main-content"><LoadingState label="Loading game lobby" /></main>
  if (!detail) return <main className="main-content"><div className="page-stack"><PageHeader eyebrow="Game lobby" title="Lobby unavailable" description={error || 'This game lobby could not be loaded.'} /><Link className="button button-secondary" to="/games">Back to tables</Link></div></main>

  const { game, participants, spectators } = detail
  const settings = game.settings ?? DEFAULT_GAME_SETTINGS
  const seatCount = Math.min(8, settings.botsEnabled ? settings.maxPlayers : Math.max(participants.length, 1))
  const seats = Array.from({ length: seatCount }, (_, index) => participants[index] ?? null)
  const canStart = game.host && game.status === 'waiting'
  const canSpectate = !game.joined && !game.spectating && !game.privateGame && game.status === 'in_progress'

  return <main className="main-content">
    <div className="page-stack game-lobby-page">
      <Link className="text-link game-lobby-back" to="/games">← Back to tables</Link>
      <PageHeader eyebrow="Game lobby" title={game.creatorUsername ? `${game.creatorUsername}’s table` : 'Game lobby'} description="Review the players, invite code and rules before entering the Uno 3D table." />
      {error ? <div className="form-alert" role="alert">{error}</div> : null}

      <div className="game-lobby-grid">
        <Card className="game-lobby-main-card">
          <div className="game-lobby-title-row"><div><span className="eyebrow">{game.privateGame ? 'Private table' : 'Public table'}</span><h2>Players in this game</h2></div><Badge tone={statusTone(game.status)}>{statusLabel(game.status)}</Badge></div>
          <div className="game-lobby-seat-grid" aria-label={`${seatCount} player seat${seatCount === 1 ? '' : 's'}`}>
            {seats.map((participant, index) => <div className={`game-lobby-seat${participant ? ' is-occupied' : ' is-open'}`} key={participant?.userId ?? `seat-${index}`}>
              <span className="game-lobby-seat-number">#{index + 1}</span>
              {participant ? <><UserAvatar username={participant.username} className="small" /><strong>{participant.username}</strong>{participant.host ? <Badge tone="warning">Host</Badge> : null}</> : <><span className="game-lobby-seat-icon">＋</span><span className="muted">{settings.botsEnabled ? 'Bot seat' : 'Open seat'}</span></>}
            </div>)}
          </div>
          <div className="game-lobby-spectators"><span className="eyebrow">Spectators</span><strong>{game.spectatorCount}</strong><span className="muted">watching this table</span>{spectators.length > 0 ? <span className="muted">{spectators.map((spectator) => spectator.username).join(', ')}</span> : null}</div>
          <div className="game-lobby-actions">{canStart ? <Button onClick={() => void startGame()} disabled={busy}>Start</Button> : null}{canSpectate ? <Button variant="secondary" onClick={() => void spectate()} disabled={busy}>Spectate game</Button> : null}{game.joined && game.status === 'waiting' ? <Button variant="secondary" onClick={() => void leave()} disabled={busy}>Leave table</Button> : null}</div>
        </Card>

        <Card className="game-lobby-code-card">
          <span className="eyebrow">Invite code</span>
          <h2>{game.privateGame ? 'Share this code' : 'Open table'}</h2>
          {game.joinCode ? <><div className="game-lobby-code"><strong>{game.joinCode}</strong><Button variant="secondary" onClick={() => void copyCode()}>{copied ? 'Copied' : 'Copy code'}</Button></div><p className="muted">Your friends can paste this code in the Tables page to join.</p></> : <p className="muted">This public table does not need an invite code.</p>}
        </Card>
      </div>

      <Card className="game-lobby-rules-card"><div className="game-lobby-title-row"><div><span className="eyebrow">Rules</span><h2>Table settings</h2></div><span className="muted">Custom games do not affect ELO.</span></div><div className="game-lobby-rules-grid">{rules.map(([label, value]) => <div className="game-lobby-rule" key={label}><span className="muted">{label}</span><strong>{value}</strong></div>)}</div></Card>
    </div>
  </main>
}
