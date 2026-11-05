import { FormEvent, useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../api/client'
import { GameRuleToggle as RuleToggle } from '../components/GameRuleToggle'
import { Badge, Button, Card, EmptyState, Input, LinkIcon, LoadingState, PageHeader, UserAvatar } from '../components/ui'
import { CUSTOM_HAND_OPTIONS, CUSTOM_PLAYER_OPTIONS, DEFAULT_GAME_SETTINGS, DRAW_PENALTY_OPTIONS } from '../gameSettings'
import type { Game, GameDetail, GameSettings } from '../types'
import { requestErrorMessage } from '../utils/errors'
import { formatDateTime } from '../utils/format'

type StatusFilter = 'all' | Game['status']

function statusLabel(status: Game['status']) {
  return status === 'in_progress' ? 'In progress' : status[0].toUpperCase() + status.slice(1)
}

function statusTone(status: Game['status']) {
  return status === 'waiting' ? 'warning' : status === 'finished' ? 'neutral' : 'success'
}

function settingsOf(game: Game) {
  return game.settings ?? DEFAULT_GAME_SETTINGS
}

function LobbyCard({ game, onInspect, onJoin, onSpectate, onDelete, busy }: { game: Game; onInspect: () => void; onJoin: () => void; onSpectate: () => void; onDelete: () => void; busy: boolean }) {
  const settings = settingsOf(game)
  return <article className={`lobby-directory-card ${game.joined ? 'joined' : ''}`}>
    <div className="lobby-directory-top"><div className="lobby-directory-title"><UserAvatar username={game.creatorUsername ?? '?'} /><div><h2>{game.creatorUsername ?? 'Open lobby'}</h2></div></div><Badge tone={statusTone(game.status)}>{statusLabel(game.status)}</Badge></div>
    <div className="lobby-directory-meta"><span>{game.privateGame ? '● Invite only' : '○ Public table'}</span><span>{game.participantCount}/{settings.maxPlayers} players</span><span>{game.spectatorCount} watching</span><span>{game.turnTimeLimit}s turns</span></div>
    <div className="lobby-directory-rules"><span>Hand {settings.startingHandSize}</span>{settings.stackDrawCards ? <span>Stacking</span> : null}{settings.jumpIn ? <span>Jump-in</span> : null}{settings.sevenZero ? <span>7–0</span> : null}{settings.botsEnabled ? <span>Bots enabled</span> : null}</div>
    <div className="lobby-directory-footer"><span className="muted">{formatDateTime(game.createdAt)}</span><div className="lobby-actions">{game.host ? <button className="text-button" onClick={onInspect}>Manage</button> : game.joined ? <button className="text-button" onClick={onInspect}>View table</button> : game.status === 'waiting' && !game.privateGame ? <button className="text-button" disabled={busy} onClick={onJoin}>Join table</button> : game.status === 'in_progress' && !game.privateGame ? <button className="text-button" disabled={busy} onClick={onSpectate}>Spectate</button> : game.status === 'waiting' ? <button className="text-button" onClick={onInspect}>Details</button> : null}{game.host && game.status === 'waiting' ? <button className="text-button danger-text" disabled={busy} onClick={onDelete}>Delete</button> : null}</div></div>
  </article>
}

export function GamesPage() {
  const PAGE_SIZE = 20
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [games, setGames] = useState<Game[]>([])
  const [turnTimeLimit, setTurnTimeLimit] = useState('15')
  const [privateGame, setPrivateGame] = useState(false)
  const [settings, setSettings] = useState<GameSettings>(DEFAULT_GAME_SETTINGS)
  const [joinCode, setJoinCode] = useState('')
  const [whitelistUsername, setWhitelistUsername] = useState('')
  const [selectedLobby, setSelectedLobby] = useState<GameDetail | null>(null)
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('waiting')
  const [search, setSearch] = useState('')
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)
  const [detailLoading, setDetailLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [page, setPage] = useState(1)
  const [hasMore, setHasMore] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')

  const load = async (reset = true) => {
    setError('')
    try {
      const nextGames = await api.listGames(undefined, reset ? 1 : page, PAGE_SIZE)
      setGames(nextGames)
      setPage(reset ? 1 : page)
      setHasMore(nextGames.length === PAGE_SIZE)
    } catch (requestError) { setError(requestErrorMessage(requestError, 'Could not load lobbies.')) } finally { setLoading(false) }
  }

  const loadMore = async () => {
    if (loadingMore || !hasMore) return
    setLoadingMore(true)
    try {
      const nextPage = page + 1
      const nextGames = await api.listGames(undefined, nextPage, PAGE_SIZE)
      setGames((current) => [...current, ...nextGames])
      setPage(nextPage)
      setHasMore(nextGames.length === PAGE_SIZE)
    } catch (requestError) {
      setError(requestErrorMessage(requestError, 'Could not load more lobbies.'))
    } finally {
      setLoadingMore(false)
    }
  }

  const inspect = async (id: string) => {
    setDetailLoading(true); setError('')
    try { setSelectedLobby(await api.getGame(id)) } catch (requestError) { setError(requestErrorMessage(requestError, 'Could not load this lobby.')) } finally { setDetailLoading(false) }
  }

  const refreshSelected = async (id: string) => { await Promise.all([load(), inspect(id)]) }
  useEffect(() => {
    void load()
    const gameId = searchParams.get('game')
    if (gameId) void inspect(gameId)
    const interval = window.setInterval(() => void load(), 5000)
    return () => window.clearInterval(interval)
  }, [searchParams])

  const create = async (event: FormEvent) => {
    event.preventDefault(); setBusy(true); setError(''); setMessage('')
    try {
      const created = await api.createGame(Number(turnTimeLimit), privateGame, 'custom', settings)
      setMessage(privateGame ? `Private lobby created. Share code ${created.joinCode}.` : 'Custom lobby created.')
      navigate(`/games/${encodeURIComponent(created.id)}`)
    } catch (requestError) { setError(requestErrorMessage(requestError, 'Could not create the lobby.')) } finally { setBusy(false) }
  }

  const joinPrivate = async (event: FormEvent) => {
    event.preventDefault(); setBusy(true); setError(''); setMessage('')
    try { const joined = await api.joinGameByCode(joinCode.trim().toUpperCase()); setJoinCode(''); navigate(`/games/${encodeURIComponent(joined.id)}`) } catch (requestError) { setError(requestErrorMessage(requestError, 'Could not join the private lobby.')) } finally { setBusy(false) }
  }

  const joinPublic = async (id: string) => {
    setBusy(true); setError(''); setMessage('')
    const previousGames = games
    setGames((current) => current.map((game) => game.id === id ? { ...game, joined: true, participantCount: game.participantCount + 1 } : game))
    try { await api.joinGame(id); navigate(`/games/${encodeURIComponent(id)}`) } catch (requestError) { setGames(previousGames); setError(requestErrorMessage(requestError, 'Could not join the lobby.')) } finally { setBusy(false) }
  }

  const spectate = async (id: string) => {
    setBusy(true); setError(''); setMessage('')
    try {
      await api.spectateGame(id)
      navigate(`/uno?game=${encodeURIComponent(id)}&spectate=1`)
    } catch (requestError) { setError(requestErrorMessage(requestError, 'Could not join spectator mode.')) } finally { setBusy(false) }
  }

  const leave = async () => {
    if (!selectedLobby) return
    setBusy(true); setError(''); setMessage('')
    const gameId = selectedLobby.game.id
    const previousGames = games
    const previousSelectedLobby = selectedLobby
    setGames((current) => current.map((game) => game.id === gameId ? { ...game, joined: false, participantCount: Math.max(0, game.participantCount - 1) } : game))
    setSelectedLobby(null)
    try { await api.leaveGame(gameId); setMessage('You left the lobby.'); await load() } catch (requestError) { setGames(previousGames); setSelectedLobby(previousSelectedLobby); setError(requestErrorMessage(requestError, 'Could not leave the lobby.')) } finally { setBusy(false) }
  }

  const remove = async (id: string) => {
    setBusy(true); setError(''); setMessage('')
    const previousGames = games
    const previousSelectedLobby = selectedLobby
    setGames((current) => current.filter((game) => game.id !== id))
    if (selectedLobby?.game.id === id) setSelectedLobby(null)
    try { await api.deleteGame(id); setMessage('Lobby deleted.'); await load() } catch (requestError) { setGames(previousGames); setSelectedLobby(previousSelectedLobby); setError(requestErrorMessage(requestError, 'Could not delete the lobby.')) } finally { setBusy(false) }
  }

  const kick = async (userId: string) => {
    if (!selectedLobby) return
    setBusy(true); setError(''); setMessage('')
    try { await api.kickParticipant(selectedLobby.game.id, userId); setMessage('Player removed from the lobby.'); await refreshSelected(selectedLobby.game.id) } catch (requestError) { setError(requestErrorMessage(requestError, 'Could not remove this player.')) } finally { setBusy(false) }
  }

  const addWhitelist = async (event: FormEvent) => {
    event.preventDefault(); if (!selectedLobby || !whitelistUsername.trim()) return
    setBusy(true); setError(''); setMessage('')
    try { await api.addWhitelist(selectedLobby.game.id, whitelistUsername.trim()); setWhitelistUsername(''); setMessage('Player added to the whitelist.'); await refreshSelected(selectedLobby.game.id) } catch (requestError) { setError(requestErrorMessage(requestError, 'Could not update the whitelist.')) } finally { setBusy(false) }
  }

  const removeWhitelist = async (userId: string) => {
    if (!selectedLobby) return
    setBusy(true); setError(''); setMessage('')
    try { await api.removeWhitelist(selectedLobby.game.id, userId); setMessage('Player removed from the whitelist.'); await refreshSelected(selectedLobby.game.id) } catch (requestError) { setError(requestErrorMessage(requestError, 'Could not update the whitelist.')) } finally { setBusy(false) }
  }

  const rotateCode = async () => {
    if (!selectedLobby) return
    setBusy(true); setError(''); setMessage('')
    try { const updated = await api.rotateGameCode(selectedLobby.game.id); setMessage(`Lobby code rotated to ${updated.joinCode}.`); await refreshSelected(selectedLobby.game.id) } catch (requestError) { setError(requestErrorMessage(requestError, 'Could not rotate the lobby code.')) } finally { setBusy(false) }
  }

  const startGame = async () => {
    if (!selectedLobby) return
    setBusy(true); setError(''); setMessage('')
    try {
      await api.startUnoGame(selectedLobby.game.id)
      navigate(`/uno?game=${encodeURIComponent(selectedLobby.game.id)}`)
    } catch (requestError) { setError(requestErrorMessage(requestError, 'Could not start the game.')) } finally { setBusy(false) }
  }

  const visibleGames = useMemo(() => {
    const query = search.trim().toLowerCase()
    return games.filter((game) => (statusFilter === 'all' || game.status === statusFilter) && (!query || `${game.creatorUsername ?? ''} ${game.mode}`.toLowerCase().includes(query)))
  }, [games, search, statusFilter])

  const updateSetting = <K extends keyof GameSettings>(key: K, value: GameSettings[K]) => setSettings((current) => ({ ...current, [key]: value }))
  const updateMaxPlayers = (value: number) => setSettings((current) => ({ ...current, maxPlayers: value, allowMultipleDecks: value >= 7 ? true : current.allowMultipleDecks }))

  return <div className="page-stack games-page">
    <PageHeader eyebrow="Custom games" title="Tables" description="Build a table with the exact rules you want. Custom games are always unranked and never touch your ELO." />
    <div className="custom-game-layout">
      <Card className="custom-setup-card"><div className="section-heading"><div><span className="eyebrow">Table builder</span><h2>Create a custom game</h2><p className="page-description">Choose the lobby capacity, turn pace and rules before inviting players.</p></div></div>
        <form className="custom-setup-form" onSubmit={create}>
          <div className="setup-field-grid"><Input id="turn-time" label="Turn limit" type="number" min={5} max={120} value={turnTimeLimit} onChange={(event) => setTurnTimeLimit(event.target.value)} /><label className="field"><span className="field-label">Max players</span><select className="input" value={settings.maxPlayers} onChange={(event) => updateMaxPlayers(Number(event.target.value))}>{CUSTOM_PLAYER_OPTIONS.map((count) => <option key={count} value={count}>{count} players</option>)}</select></label><label className="field"><span className="field-label">Starting hand</span><select className="input" value={settings.startingHandSize} onChange={(event) => updateSetting('startingHandSize', Number(event.target.value))}>{CUSTOM_HAND_OPTIONS.map((count) => <option key={count} value={count}>{count} cards</option>)}</select></label><label className="field"><span className="field-label">Draw penalty</span><select className="input" value={settings.drawPenalty} onChange={(event) => updateSetting('drawPenalty', Number(event.target.value))}>{DRAW_PENALTY_OPTIONS.map((count) => <option key={count} value={count}>+{count} cards</option>)}</select></label></div>
          <div className="game-privacy-row"><label className="switch-field"><input type="checkbox" checked={privateGame} onChange={(event) => setPrivateGame(event.target.checked)} /><span><strong>Private invite-only table</strong><small>Generate a six-character code</small></span></label><span className="muted">The host can kick players and manage the whitelist.</span></div>
          <div className="house-rules"><div className="house-rules-heading"><div><span className="eyebrow">Rules</span><strong>Optional rule set</strong></div><span className="muted">The game service will enforce these settings.</span></div><div className="house-rules-grid"><RuleToggle label="Stack draw cards" hint="Pass +2 / +4 penalties forward" checked={settings.stackDrawCards} onChange={(value) => updateSetting('stackDrawCards', value)} /><RuleToggle label="Jump-in" hint="Play an identical card out of turn" checked={settings.jumpIn} onChange={(value) => updateSetting('jumpIn', value)} /><RuleToggle label="Seven–zero" hint="7 swaps hands, 0 rotates them" checked={settings.sevenZero} onChange={(value) => updateSetting('sevenZero', value)} /><RuleToggle label="Draw until playable" hint="Keep drawing until a card fits" checked={settings.drawUntilPlayable} onChange={(value) => updateSetting('drawUntilPlayable', value)} /><RuleToggle label="Force play" hint="Play the drawn card when valid" checked={settings.forcePlay} onChange={(value) => updateSetting('forcePlay', value)} /><RuleToggle label="Challenge +4" hint="Allow the official challenge rule" checked={settings.challengeDrawFour} onChange={(value) => updateSetting('challengeDrawFour', value)} /><RuleToggle label="Multiple decks" hint="Use extra decks for larger tables" checked={settings.allowMultipleDecks} onChange={(value) => updateSetting('allowMultipleDecks', value)} /><RuleToggle label="Allow bots" hint="Fill open seats when the host starts" checked={settings.botsEnabled} onChange={(value) => updateSetting('botsEnabled', value)} /></div></div>
          <div className="custom-setup-actions"><Button type="submit" disabled={busy}>{busy ? 'Creating…' : privateGame ? 'Create private table' : 'Create public table'}</Button><span className="muted">Custom rules do not affect competitive rank.</span></div>
        </form>
      </Card>
      <Card className="join-code-card"><div><span className="eyebrow">Invite code</span><h2>Join a friend</h2><p className="muted">Enter the code shared by the host. Private tables can also use a whitelist.</p></div><form className="code-form" onSubmit={joinPrivate}><input className="input lobby-code-input" value={joinCode} onChange={(event) => setJoinCode(event.target.value.replace(/[^a-z0-9]/gi, '').slice(0, 6).toUpperCase())} placeholder="ABC123" aria-label="Private lobby code" maxLength={6} required /><Button variant="secondary" type="submit" disabled={busy}>Join table</Button></form><div className="join-code-note"><LinkIcon className="ui-icon" size={16} aria-hidden="true" /><span>Codes can be rotated by the host at any time.</span></div></Card>
    </div>
    {message ? <div className="success-alert" role="status">{message}</div> : null}{error ? <div className="form-alert" role="alert">{error}</div> : null}
    <Card className="lobby-directory-card-wrap"><div className="section-heading"><div><span className="eyebrow">Custom tables</span><h2>Find a table</h2><p className="page-description">Only custom games appear here. Ranked and unranked games start automatically through matchmaking.</p></div></div><div className="lobby-directory-toolbar"><input className="input" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search by host" aria-label="Search custom tables" /><div className="directory-filters">{(['waiting', 'all', 'in_progress', 'finished'] as StatusFilter[]).map((filter) => <button key={filter} className={`directory-filter ${statusFilter === filter ? 'active' : ''}`} onClick={() => setStatusFilter(filter)}>{filter === 'all' ? 'All' : statusLabel(filter)}</button>)}</div></div>{loading ? <LoadingState label="Loading tables" /> : visibleGames.length === 0 ? <EmptyState title="No custom tables match" description="Create a custom table or adjust the filters to find another lobby." /> : <><div className="lobby-directory-grid">{visibleGames.map((game) => <LobbyCard key={game.id} game={game} onInspect={() => navigate(`/games/${encodeURIComponent(game.id)}`)} onJoin={() => void joinPublic(game.id)} onSpectate={() => void spectate(game.id)} onDelete={() => void remove(game.id)} busy={busy} />)}</div>{hasMore ? <div className="load-more-row"><button className="button button-secondary" type="button" onClick={() => void loadMore()} disabled={loadingMore}>{loadingMore ? 'Loading…' : 'Load more tables'}</button></div> : null}</>}</Card>
    {detailLoading ? <Card><LoadingState label="Loading table" /></Card> : null}
    {selectedLobby ? <Card className="lobby-control-card"><div className="lobby-control-header"><div><span className="eyebrow">{selectedLobby.game.host ? 'Host controls' : 'Table details'}</span><h2>{selectedLobby.game.creatorUsername ?? 'Lobby'}’s table</h2><p className="muted">{selectedLobby.game.participantCount}/{settingsOf(selectedLobby.game).maxPlayers} players · {statusLabel(selectedLobby.game.status)} · {selectedLobby.game.mode}</p></div><div className="lobby-header-actions">{selectedLobby.game.host && selectedLobby.game.privateGame ? <><span className="lobby-code"><small>Invite code</small><strong>{selectedLobby.game.joinCode}</strong></span><Button variant="secondary" onClick={() => void rotateCode()} disabled={busy}>Rotate code</Button></> : null}{selectedLobby.game.host && selectedLobby.game.status === 'waiting' ? <Button onClick={() => void startGame()} disabled={busy}>Start</Button> : null}{selectedLobby.game.joined && selectedLobby.game.status === 'in_progress' ? <Link className="button button-secondary" to={`/uno?game=${encodeURIComponent(selectedLobby.game.id)}`}>Start</Link> : null}<button className="text-button" onClick={() => setSelectedLobby(null)}>Close</button></div></div><div className="lobby-settings-summary"><span><strong>{settingsOf(selectedLobby.game).startingHandSize}</strong> starting cards</span><span><strong>+{settingsOf(selectedLobby.game).drawPenalty}</strong> draw penalty</span><span><strong>{settingsOf(selectedLobby.game).stackDrawCards ? 'On' : 'Off'}</strong> stacking</span><span><strong>{settingsOf(selectedLobby.game).jumpIn ? 'On' : 'Off'}</strong> jump-in</span><span><strong>{settingsOf(selectedLobby.game).botsEnabled ? 'On' : 'Off'}</strong> bots</span></div>
      <div className="lobby-control-grid"><div className="lobby-control-section"><div className="section-heading"><div><span className="eyebrow">In this table</span><h2>Participants</h2></div><span className="badge badge-neutral">{selectedLobby.participants.length} players</span></div><div className="participant-list">{selectedLobby.participants.map((participant) => <div className="participant-row" key={participant.userId}><div className="table-player"><UserAvatar username={participant.username} className="small" /><Link className="text-link" to={`/players/${participant.userId}`}>{participant.username}</Link>{participant.host ? <Badge tone="warning">Host</Badge> : null}</div>{selectedLobby.game.host && !participant.host ? <button className="text-button danger-text" disabled={busy} onClick={() => void kick(participant.userId)}>Kick</button> : null}</div>)}</div></div>{selectedLobby.game.host ? <div className="lobby-control-section"><div className="section-heading"><div><span className="eyebrow">Access control</span><h2>Whitelist</h2></div><span className="badge badge-neutral">{selectedLobby.whitelist.length} allowed</span></div><p className="lobby-note muted">Adding a player activates an allow-list for this private table.</p><form className="whitelist-form" onSubmit={addWhitelist}><input className="input" value={whitelistUsername} onChange={(event) => setWhitelistUsername(event.target.value)} placeholder="Username" maxLength={24} aria-label="Username to whitelist" required /><Button type="submit" variant="secondary" disabled={busy}>Add player</Button></form><div className="whitelist-list">{selectedLobby.whitelist.length === 0 ? <span className="muted lobby-empty-note">No whitelist restriction yet.</span> : selectedLobby.whitelist.map((entry) => <div className="participant-row" key={entry.userId}><Link className="text-link" to={`/players/${entry.userId}`}>{entry.username}</Link><button className="text-button danger-text" disabled={busy} onClick={() => void removeWhitelist(entry.userId)}>Remove</button></div>)}</div></div> : <div className="lobby-control-section lobby-member-actions"><span className="eyebrow">Your access</span><h2>{selectedLobby.game.joined ? 'You are in this table.' : selectedLobby.game.spectating ? 'You are spectating.' : 'Public table'}</h2><p className="muted">{selectedLobby.game.spectating ? 'You can watch this public game without occupying a player seat.' : 'Players reconnect through their own account. Public games can be watched without joining.'}</p>{selectedLobby.game.joined ? <Button variant="secondary" onClick={() => void leave()} disabled={busy}>Leave table</Button> : selectedLobby.game.status === 'in_progress' && !selectedLobby.game.privateGame ? <Button variant="secondary" onClick={() => void spectate(selectedLobby.game.id)} disabled={busy}>Spectate game</Button> : null}</div>}</div>
    </Card> : null}
  </div>
}
