import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api/client'
import { PlayerAnalytics } from '../components/PlayerAnalytics'
import type { User, UserAnalytics } from '../types'
import { ArrowLeft, Card, EmptyState, LoadingState, PageHeader, StatCard, UserAvatar } from '../components/ui'
import { requestErrorMessage } from '../utils/errors'
import { formatDate } from '../utils/format'
import { usePagePolling } from '../utils/polling'

export function PublicProfilePage() {
  const { id } = useParams<{ id: string }>()
  const [profile, setProfile] = useState<User | null>(null)
  const [analytics, setAnalytics] = useState<UserAnalytics | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [analyticsError, setAnalyticsError] = useState('')

  const loadProfile = useCallback(() => {
    if (!id) return
    api.getPublicProfile(id)
      .then(setProfile)
      .catch((requestError) => setError(requestErrorMessage(requestError, 'Could not load this profile.')))
      .finally(() => setLoading(false))
    api.getUserAnalytics(id)
      .then(setAnalytics)
      .catch((requestError) => setAnalyticsError(requestErrorMessage(requestError, 'Could not load player analytics.')))
  }, [id])

  useEffect(() => {
    if (!id) return undefined
    loadProfile()
    window.addEventListener('uno-ranked:elo-updated', loadProfile)
    return () => {
      window.removeEventListener('uno-ranked:elo-updated', loadProfile)
    }
  }, [id, loadProfile])
  usePagePolling(loadProfile, 5000, Boolean(id))

  if (loading) return <LoadingState label="Loading profile" />
  if (error || !profile) return <div className="page-stack"><PageHeader title="Profile unavailable" description={error || 'This player could not be found.'} /><Card><EmptyState title="No profile to show" description="The account may have been removed or the link is invalid." /><Link className="button button-secondary" to="/leaderboard">Back to leaderboard</Link></Card></div>

  const winRate = profile.stats.gamesPlayed === 0 ? 0 : Math.round((profile.stats.gamesWon / profile.stats.gamesPlayed) * 100)
  return <div className="page-stack public-profile-page">
    <div className="public-profile-header"><Link className="button button-secondary" to="/leaderboard"><ArrowLeft className="ui-icon" size={15} aria-hidden="true" />Leaderboard</Link></div>
    <div className="public-profile-hero">
      <div className="public-profile-identity"><UserAvatar username={profile.username} avatarUrl={profile.avatarUrl} className="large" /><div><h1>{profile.username}</h1><span className="muted">Member since {formatDate(profile.createdAt)}</span></div></div>
      <div className="public-profile-rating"><span>ELO</span><strong>{profile.elo}</strong></div>
    </div>
    <Card className="public-profile-bio"><span className="eyebrow">About</span><p>{profile.bio || 'No bio yet.'}</p></Card>
    <div className="stats-grid public-profile-stats">
      <StatCard label="Games" value={profile.stats.gamesPlayed} detail="Played" />
      <StatCard label="Wins" value={profile.stats.gamesWon} detail={winRate + '% win rate'} />
      <StatCard label="Streak" value={profile.stats.currentStreak} detail={'Best: ' + profile.stats.maxStreak} />
      <StatCard label="UNO" value={profile.stats.unosCalled} detail="Calls" />
    </div>
    <Card className="profile-record">
      <div className="profile-record-heading"><h2>Stats</h2></div>
      <div className="profile-record-grid">
        <div className="profile-record-stat"><strong>{profile.stats.unoPenalties}</strong><span>UNO penalties</span></div>
        <div className="profile-record-stat"><strong>{profile.stats.drawFoursPlayed}</strong><span>+4 played</span></div>
        <div className="profile-record-stat"><strong>{profile.stats.cardsDrawn}</strong><span>Cards drawn</span></div>
        <div className="profile-record-stat"><strong>{formatDate(profile.lastLogin)}</strong><span>Last seen</span></div>
      </div>
    </Card>
    {analyticsError ? <div className="form-alert" role="alert">{analyticsError}</div> : null}
    {analytics ? <PlayerAnalytics analytics={analytics} /> : <Card><LoadingState label="Loading player analytics" /></Card>}
  </div>
}
