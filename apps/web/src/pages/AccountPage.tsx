import { FormEvent, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { Button, Card, Input, PageHeader } from '../components/ui'
import { requestErrorMessage } from '../utils/errors'

export function AccountPage() {
  const { user, signOut } = useAuth()
  const navigate = useNavigate()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  if (!user) return null

  const changePassword = async (event: FormEvent) => {
    event.preventDefault()
    setMessage('')
    setError('')
    if (newPassword !== confirmation) {
      setError('The new passwords do not match.')
      return
    }
    setBusy(true)
    try {
      await api.changePassword(currentPassword, newPassword)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmation('')
      setMessage('Password updated.')
    } catch (requestError) {
      setError(requestErrorMessage(requestError, 'Could not update your password.'))
    } finally {
      setBusy(false)
    }
  }

  const deleteAccount = async () => {
    if (!window.confirm('Delete your account and all linked data? This cannot be undone.')) return
    try {
      await api.deleteProfile()
      await signOut()
      navigate('/dashboard', { replace: true })
    } catch (requestError) {
      setError(requestErrorMessage(requestError, 'Could not delete your account.'))
    }
  }

  return <div className="page-stack">
    <PageHeader eyebrow="Account" title="Account" description="Credentials and account access." />
    <div className="account-grid">
      <Card>
        <div className="section-heading"><h2>Account information</h2></div>
        <div className="stack-form">
          <Input id="account-username" label="Username" value={user.username} readOnly hint="Username cannot be changed." />
          <Input id="account-email" label="Email" type="email" value={user.email ?? ''} readOnly hint="Your 42 email cannot be changed." />
        </div>
      </Card>
      <Card>
        <div className="section-heading"><h2>Change password</h2></div>
        <form className="stack-form" onSubmit={(event) => void changePassword(event)} noValidate>
          {message ? <div className="success-alert" role="status">{message}</div> : null}
          {error ? <div className="form-alert" role="alert">{error}</div> : null}
          <Input id="current-password" label="Current password" type="password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} autoComplete="current-password" required />
          <Input id="new-password" label="New password" type="password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} minLength={12} maxLength={128} autoComplete="new-password" required hint="At least 12 characters." />
          <Input id="confirm-password" label="Confirm new password" type="password" value={confirmation} onChange={(event) => setConfirmation(event.target.value)} minLength={12} maxLength={128} autoComplete="new-password" required />
          <Button type="submit" disabled={busy}>{busy ? 'Updating…' : 'Update password'}</Button>
        </form>
      </Card>
      <Card className="danger-card account-danger-card">
        <div className="section-heading"><h2>Delete account</h2></div>
        <p className="muted">This removes your account, sessions and linked statistics.</p>
        <Button variant="danger" onClick={() => void deleteAccount()}>Delete my account</Button>
      </Card>
    </div>
  </div>
}
