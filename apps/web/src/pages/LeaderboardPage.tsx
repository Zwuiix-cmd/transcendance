import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import type { LeaderboardEntry, ProfileSearchResult } from '../types'
import { Badge, Card, EmptyState, LoadingState, Search, UserAvatar } from '../components/ui'
import { usePagePolling } from '../utils/polling'
function PodiumCard({ entry }: { entry: LeaderboardEntry }) {
  return <Link to={`/players/${entry.userId}`} className={`podium-card podium-${entry.rank}`}><span className="podium-place">#{entry.rank}</span><UserAvatar username={entry.username} avatarUrl={entry.avatarUrl} className="podium-avatar" /><strong>{entry.username}</strong><span className="podium-elo">{entry.elo} ELO</span><span className="muted">{Math.round(entry.winRate * 100)}% win rate</span></Link>
}

function SearchResult({ profile }: { profile: ProfileSearchResult }) {
  return <Link className="profile-search-result" to={`/players/${profile.id}`}><UserAvatar username={profile.username} avatarUrl={profile.avatarUrl} className="small" /><span><strong>{profile.username}</strong><small>{profile.elo} ELO</small></span></Link>
}

export function LeaderboardPage() {
  const PAGE_SIZE = 100
  const { user } = useAuth()
  const [entries, setEntries] = useState<LeaderboardEntry[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<ProfileSearchResult[]>([])
  const [loading, setLoading] = useState(true)
  const [searchLoading, setSearchLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [page, setPage] = useState(1)
  const [hasMore, setHasMore] = useState(false)

  const loadLeaderboard = useCallback(() => {
    api.leaderboard(PAGE_SIZE, 1)
      .then((nextEntries) => {
        setEntries(nextEntries)
        setPage(1)
        setHasMore(nextEntries.length === PAGE_SIZE)
      })
      .catch(() => undefined)
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    loadLeaderboard()
    window.addEventListener('uno-ranked:elo-updated', loadLeaderboard)
    return () => {
      window.removeEventListener('uno-ranked:elo-updated', loadLeaderboard)
    }
  }, [loadLeaderboard])
  usePagePolling(loadLeaderboard, 5000)

  const loadMore = async () => {
    if (loadingMore || !hasMore) return
    setLoadingMore(true)
    try {
      const nextPage = page + 1
      const nextEntries = await api.leaderboard(PAGE_SIZE, nextPage)
      setEntries((current) => [...current, ...nextEntries])
      setPage(nextPage)
      setHasMore(nextEntries.length === PAGE_SIZE)
    } finally {
      setLoadingMore(false)
    }
  }

  useEffect(() => {
    const query = searchQuery.trim()
    if (query.length < 2) {
      setSearchResults([])
      setSearchLoading(false)
      return undefined
    }
    setSearchLoading(true)
    const timer = window.setTimeout(() => {
      api.searchProfiles(query)
        .then(setSearchResults)
        .catch(() => setSearchResults([]))
        .finally(() => setSearchLoading(false))
    }, 240)
    return () => window.clearTimeout(timer)
  }, [searchQuery])

  const averageElo = useMemo(() => entries.length === 0 ? 0 : Math.round(entries.reduce((total, entry) => total + entry.elo, 0) / entries.length), [entries])
  const currentUserRank = entries.find((entry) => entry.userId === user?.id)
  const podium = [entries[1], entries[0], entries[2]].filter((entry): entry is LeaderboardEntry => Boolean(entry))
  const showSearch = searchQuery.trim().length >= 2

  return <div className="page-stack leaderboard-page leaderboard-page-v2">
    <header className="leaderboard-page-header leaderboard-heading-centered">
      <div><span className="eyebrow">Leaderboard</span><h1>Top players.</h1><p className="page-description">Find a player or follow the climb.</p></div>
      <div className="profile-search-box"><label htmlFor="profile-search">Search players</label><div className="profile-search-input"><Search className="ui-icon" size={17} aria-hidden="true" /><input id="profile-search" value={searchQuery} onChange={(event) => setSearchQuery(event.target.value)} placeholder="Username" autoComplete="off" /></div>{showSearch ? <div className="profile-search-results" role="listbox">{searchLoading ? <LoadingState label="Searching" /> : searchResults.length === 0 ? <p className="profile-search-message">No player matches that username.</p> : searchResults.map((profile) => <SearchResult key={profile.id} profile={profile} />)}</div> : null}</div>
    </header>
    {loading ? <Card><LoadingState label="Loading leaderboard" /></Card> : entries.length === 0 ? <Card><EmptyState title="No leaderboard yet" description="Complete a game to start the season." /></Card> : <>
      <Card className="leaderboard-overview"><div><span className="eyebrow">Ranked board</span><h2>Climb the table.</h2><p className="muted">Ordered by ELO, then wins.</p></div><div className="leaderboard-summary"><div><strong>{entries.length}</strong><span>Players</span></div><div><strong>{averageElo}</strong><span>Average ELO</span></div>{currentUserRank ? <div><strong>#{currentUserRank.rank}</strong><span>Your place</span></div> : null}</div></Card>
      {entries.length >= 3 ? <div className={`podium-grid podium-grid-${Math.min(podium.length, 3)}`}>{podium.map((entry) => <PodiumCard key={entry.userId} entry={entry} />)}</div> : null}
      <Card className="ranking-table-card"><div className="section-heading"><div><span className="eyebrow">Rankings</span><h2>All players</h2></div><span className="muted ranking-updated">ELO</span></div><div className="table-wrap"><table className="ranking-table"><caption className="sr-only">Leaderboard</caption><thead><tr><th>Rank</th><th>Player</th><th>ELO</th><th>Win rate</th><th>Wins</th><th>Games</th></tr></thead><tbody>{entries.map((entry) => <tr key={entry.userId} className={`${entry.rank === 1 ? 'top-row' : ''} ${entry.userId === user?.id ? 'current-player-row' : ''}`}><td><span className={`rank-number ${entry.rank <= 3 ? 'rank-top' : ''}`}>#{entry.rank}</span></td><td><Link className="table-player" to={`/players/${entry.userId}`}><UserAvatar username={entry.username} avatarUrl={entry.avatarUrl} className="small" /><strong>{entry.username}</strong>{entry.userId === user?.id ? <Badge tone="success">You</Badge> : entry.rank <= 3 ? <Badge tone="warning">Top {entry.rank}</Badge> : null}</Link></td><td><strong>{entry.elo}</strong></td><td>{Math.round(entry.winRate * 100)}%</td><td>{entry.gamesWon}</td><td>{entry.gamesPlayed}</td></tr>)}</tbody></table></div>{hasMore ? <div className="load-more-row"><button className="button button-secondary" type="button" onClick={() => void loadMore()} disabled={loadingMore}>{loadingMore ? 'Loading…' : 'Load more players'}</button></div> : null}</Card>
    </>}
  </div>
}
