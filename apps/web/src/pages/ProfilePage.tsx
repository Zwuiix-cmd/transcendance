import { ChangeEvent, FormEvent, useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { PlayerAnalytics } from '../components/PlayerAnalytics'
import type { UserAnalytics } from '../types'
import { Button, Camera, Card, LoadingState, PageHeader, StatCard, TextArea, UserAvatar } from '../components/ui'
import { requestErrorMessage } from '../utils/errors'
import { formatDate } from '../utils/format'
import { usePagePolling } from '../utils/polling'

export function ProfilePage() {
  const { user, refreshUser } = useAuth()
  const [bio, setBio] = useState(user?.bio ?? '')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [analytics, setAnalytics] = useState<UserAnalytics | null>(null)
  const [analyticsError, setAnalyticsError] = useState('')
  const [busy, setBusy] = useState(false)
  const [avatarBusy, setAvatarBusy] = useState(false)
  useEffect(() => { setBio(user?.bio ?? '') }, [user])
  const loadAnalytics = useCallback(() => {
    if (!user) return
    api.getUserAnalytics(user.id)
      .then(setAnalytics)
      .catch((requestError) => setAnalyticsError(requestErrorMessage(requestError, 'Could not load player analytics.')))
  }, [user])
  useEffect(() => {
    if (!user) return undefined
    loadAnalytics()
    window.addEventListener('uno-ranked:elo-updated', loadAnalytics)
    return () => {
      window.removeEventListener('uno-ranked:elo-updated', loadAnalytics)
    }
  }, [loadAnalytics, user])
  usePagePolling(loadAnalytics, 5000, Boolean(user))
  if (!user) return null

  const save = async (event: FormEvent) => {
    event.preventDefault(); setBusy(true); setMessage(''); setError('')
    try { await api.updateProfile(bio); await refreshUser(); setMessage('Saved.') } catch (requestError) { setError(requestErrorMessage(requestError, 'Could not update your profile.')) } finally { setBusy(false) }
  }
  const uploadAvatar = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    if (file.size > 5 * 1024 * 1024) { setError('Avatar must be 5 MB or smaller.'); return }
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) { setError('Use a PNG, JPEG or WebP image.'); return }
    setAvatarBusy(true); setMessage(''); setError('')
    try {
      await api.uploadAvatar(file)
      await refreshUser()
      setMessage('Avatar updated.')
    } catch (requestError) { setError(requestErrorMessage(requestError, 'Could not upload your avatar.')) } finally { setAvatarBusy(false) }
  }
  const winRate = user.stats.gamesPlayed === 0 ? 0 : Math.round((user.stats.gamesWon / user.stats.gamesPlayed) * 100)

  return (
    <div className="page-stack profile-page profile-page-own">
      <PageHeader title="Profile" />
      <Card className="profile-identity-card">
        <div className="profile-identity-main">
            <label className="avatar-editable" htmlFor="profile-avatar" tabIndex={0} aria-label="Change avatar" onKeyDown={(event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault()
                document.getElementById('profile-avatar')?.click()
              }
            }}>
              <UserAvatar username={user.username} avatarUrl={user.avatarUrl} className="large" />
              <span className="avatar-edit-badge" aria-hidden="true">
                <Camera className="ui-icon" size={14} aria-hidden="true" />
              </span>
            </label>
            <input id="profile-avatar" className="sr-only" type="file" accept="image/png,image/jpeg,image/webp" onChange={(event) => void uploadAvatar(event)} disabled={avatarBusy} />
            <div>
              <h2>{user.username}</h2>
              <span className="muted">Member since {formatDate(user.createdAt)}</span>
            </div>
          </div>
        <div className="profile-identity-rating"><span>ELO</span><strong>{user.elo}</strong></div>
      </Card>
      <div className="profile-grid">
        <Card className="profile-editor-card">
          <div className="profile-editor-heading"><div><span className="eyebrow">About</span><h2>Edit profile</h2></div></div>
          <form className="stack-form" onSubmit={save} noValidate>
            {message ? <div className="success-alert" role="status">{message}</div> : null}
            {error ? <div className="form-alert" role="alert">{error}</div> : null}
            <TextArea id="profile-bio" label="Bio" value={bio} onChange={(event) => setBio(event.target.value)} maxLength={180} rows={4} hint={`${bio.length}/180 · Shown on your public profile.`} placeholder="Tell the table who you are." />
            <Button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save changes'}</Button>
          </form>
        </Card>
        <div className="profile-side">
          <div className="stats-grid profile-stats">
            <StatCard label="ELO" value={user.elo} detail="Current rating" />
            <StatCard label="Games" value={user.stats.gamesPlayed} detail="Played" />
            <StatCard label="Wins" value={user.stats.gamesWon} detail={`${winRate}% win rate`} />
            <StatCard label="Win streak" value={user.stats.currentStreak} detail={`Best: ${user.stats.maxStreak}`} />
          </div>
          <Card className="profile-record">
            <div className="profile-record-heading"><h2>Stats</h2></div>
            <div className="profile-record-grid">
              <div className="profile-record-stat"><strong>{user.stats.unosCalled}</strong><span>UNO calls</span></div>
              <div className="profile-record-stat"><strong>{user.stats.unoPenalties}</strong><span>UNO penalties</span></div>
              <div className="profile-record-stat"><strong>{user.stats.drawFoursPlayed}</strong><span>+4 played</span></div>
              <div className="profile-record-stat"><strong>{user.stats.cardsDrawn}</strong><span>Cards drawn</span></div>
            </div>
          </Card>
        </div>
      </div>
      {analyticsError ? <div className="form-alert" role="alert">{analyticsError}</div> : null}
      {analytics ? <PlayerAnalytics analytics={analytics} /> : <Card><LoadingState label="Loading player analytics" /></Card>}
    </div>
  )
}
