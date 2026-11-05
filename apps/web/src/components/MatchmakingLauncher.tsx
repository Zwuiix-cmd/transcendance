import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { AuthButton } from './AuthModal'
import { ArrowRight, CardIcon, ChevronRight, Cross, Crown, Globe, Play } from './ui'
import type { MatchmakingStatus, QueueMode } from '../types'
import { usePagePolling } from '../utils/polling'

const MODES: Array<{ id: QueueMode; label: string; title: string; description: string }> = [
  { id: 'ranked', label: 'Ranked', title: 'Climb your rating', description: 'ELO changes. The search range widens while you wait.' },
  { id: 'unranked', label: 'Unranked', title: 'Play without rating', description: 'No ELO changes. Any rating can join the search.' },
]

function ModeIcon({ mode }: { mode: QueueMode | 'custom' }) {
  if (mode === 'ranked') return <Crown className="ui-icon" size={19} aria-hidden="true" />
  if (mode === 'unranked') return <Globe className="ui-icon" size={19} aria-hidden="true" />
  return <CardIcon className="ui-icon" size={19} aria-hidden="true" />
}

function emptyQueue(): MatchmakingStatus {
  return {
    status: 'not_waiting', mode: null, queued: false, matched: false, queueSize: 0, position: 0,
    waitedSeconds: 0, estimatedWaitSeconds: 0, ticketId: null, createdAt: null, players: [],
    searchEloMin: null, searchEloMax: null, searchEloGap: null,
  }
}

export function getSearchWindow(queue: MatchmakingStatus, mode: QueueMode, currentElo: number) {
  if (mode === 'unranked') return { label: 'Any ELO', detail: 'No rating filter' }
  const gap = queue.searchEloGap ?? 200
  const min = queue.searchEloMin ?? Math.max(0, currentElo - gap)
  const max = queue.searchEloMax ?? currentElo + gap
  return { label: `${min}–${max}`, detail: `±${gap} ELO` }
}

type MatchmakingContextValue = {
  queue: MatchmakingStatus
  busy: boolean
  launcherOpen: boolean
  openLauncher: () => void
  closeLauncher: () => void
  startSearch: (mode: QueueMode) => Promise<boolean>
  leaveSearch: () => Promise<void>
}

const MatchmakingContext = createContext<MatchmakingContextValue | undefined>(undefined)

export function MatchmakingProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth()
  const [queue, setQueue] = useState<MatchmakingStatus>(emptyQueue())
  const [launcherOpen, setLauncherOpen] = useState(false)
  const [busy, setBusy] = useState(false)

  const refreshQueue = useCallback(async () => {
    try {
      setQueue(await api.matchmakingStatus())
    } catch {
      // A transient status request must not interrupt the rest of the application.
    }
  }, [])

  useEffect(() => {
    if (!user) return
    void refreshQueue()
  }, [refreshQueue, user])

  useEffect(() => {
    if (user) return
    setQueue(emptyQueue())
    setLauncherOpen(false)
  }, [user])

  usePagePolling(refreshQueue, 1000, Boolean(queue.queued && !queue.matched))

  const startSearch = useCallback(async (mode: QueueMode) => {
    if (!user || busy) return false
    setBusy(true)
    try {
      setQueue(await api.joinMatchmaking(mode))
      setLauncherOpen(false)
      return true
    } catch {
      return false
    } finally {
      setBusy(false)
    }
  }, [busy, user])

  const leaveSearch = useCallback(async () => {
    if (busy) return
    setBusy(true)
    try {
      await api.leaveMatchmaking()
      setQueue(emptyQueue())
      setLauncherOpen(false)
    } catch {
      // Keep the launcher usable even if the cancellation request is delayed.
    } finally {
      setBusy(false)
    }
  }, [busy])

  const value = useMemo<MatchmakingContextValue>(() => ({
    queue,
    busy,
    launcherOpen,
    openLauncher: () => setLauncherOpen(true),
    closeLauncher: () => setLauncherOpen(false),
    startSearch,
    leaveSearch,
  }), [busy, launcherOpen, leaveSearch, queue, startSearch])

  return <MatchmakingContext.Provider value={value}>{children}</MatchmakingContext.Provider>
}

export function useMatchmaking() {
  const context = useContext(MatchmakingContext)
  if (!context) throw new Error('useMatchmaking must be used inside MatchmakingProvider')
  return context
}

export function PlayButton({ children = 'Play', className = '', onClick, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) {
  const { openLauncher } = useMatchmaking()
  return <button {...props} type="button" className={`button button-primary ${className}`} onClick={(event) => { onClick?.(event); openLauncher() }}><Play className="ui-icon" size={15} aria-hidden="true" />{children}</button>
}

function LauncherSearch() {
  const { queue, busy, leaveSearch, closeLauncher } = useMatchmaking()
  const { user } = useAuth()
  const mode = queue.mode ?? 'ranked'
  const range = getSearchWindow(queue, mode, user?.elo ?? 1000)

  return <div className="play-launcher-search">
    <div className="play-launcher-search-heading"><div><span className="eyebrow">{queue.matched ? 'Match found' : mode}</span><h3>{queue.matched ? 'Players are ready' : 'Looking for opponents'}</h3></div><span className="play-launcher-time">{queue.matched ? 'Ready' : `${queue.waitedSeconds}s`}</span></div>
    <div className="play-launcher-range"><span>Search window</span><strong>{range.label}</strong><small>{range.detail}{queue.matched ? '' : ' · expands automatically'}</small></div>
    {!queue.matched ? <div className="play-launcher-metrics"><span><small>Position</small><strong>#{queue.position}</strong></span><span><small>Players</small><strong>{queue.queueSize}</strong></span><span><small>Expected</small><strong>{queue.estimatedWaitSeconds}s</strong></span></div> : <p className="play-launcher-note">The match ticket is ready for the Uno game service.</p>}
    <div className="play-launcher-actions"><button className="text-button" type="button" onClick={() => void leaveSearch()} disabled={busy}>{queue.matched ? 'Leave match' : 'Cancel search'}</button><button className="text-button" type="button" onClick={closeLauncher}>Close</button></div>
  </div>
}

export function MatchmakingLauncher() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const { queue, busy, launcherOpen, openLauncher, closeLauncher, startSearch } = useMatchmaking()
  const mode = queue.mode ?? 'ranked'
  const range = getSearchWindow(queue, mode, user?.elo ?? 1000)

  const open = () => {
    openLauncher()
  }

  const openCustomGames = () => {
    closeLauncher()
    navigate('/games')
  }

  return <>
    {queue.queued ? <button className={`matchmaking-toast ${queue.matched ? 'matchmaking-toast-matched' : ''}`} type="button" onClick={open} aria-label="Open matchmaking status">
      <span className="matchmaking-toast-dot" aria-hidden="true" />
      <span className="matchmaking-toast-copy"><strong>{queue.matched ? 'Match found' : `Searching ${mode}`}</strong><small>{queue.matched ? 'Open match details' : `${range.label} ELO · ${queue.waitedSeconds}s`}</small></span>
      <ArrowRight className="matchmaking-toast-arrow ui-icon" size={16} aria-hidden="true" />
    </button> : null}
    {launcherOpen ? <div className="play-launcher-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) closeLauncher() }}>
      <section className="play-launcher" role="dialog" aria-modal="true" aria-labelledby="play-launcher-title">
        <div className="play-launcher-heading"><div><h2 id="play-launcher-title">{queue.queued ? (queue.matched ? 'Match found' : 'Matchmaking') : 'Choose a match'}</h2></div><button className="dialog-close" type="button" onClick={closeLauncher} aria-label="Close"><Cross className="ui-icon" size={17} aria-hidden="true" /></button></div>
        {queue.queued ? <LauncherSearch /> : user ? <>
          <div className="play-launcher-options" aria-label="Match type">
            {MODES.map((option) => <button key={option.id} className="play-launcher-option" type="button" onClick={() => void startSearch(option.id)} disabled={busy}><span className="play-launcher-option-icon"><ModeIcon mode={option.id} /></span><span><strong>{option.label}</strong><small>{option.title}</small><em>{option.description}</em></span><ChevronRight className="ui-icon play-launcher-option-arrow" size={17} aria-hidden="true" /></button>)}
            <button className="play-launcher-option play-launcher-custom" type="button" onClick={openCustomGames} disabled={busy}><span className="play-launcher-option-icon"><ModeIcon mode="custom" /></span><span><strong>Custom</strong><small>Play with friends</small><em>Set the table size, privacy and house rules.</em></span><ChevronRight className="ui-icon play-launcher-option-arrow" size={17} aria-hidden="true" /></button>
          </div>
        </> : <div className="play-launcher-guest"><h3>Sign in to play</h3><p>Choose a mode after you connect your account.</p><div className="play-launcher-guest-actions"><AuthButton mode="login" className="button-primary" onClick={closeLauncher}>Sign in</AuthButton><AuthButton mode="register" className="button-secondary" onClick={closeLauncher}>Create account</AuthButton></div></div>}
      </section>
    </div> : null}
  </>
}
