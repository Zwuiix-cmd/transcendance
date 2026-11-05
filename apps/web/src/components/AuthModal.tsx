import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import type { ButtonHTMLAttributes, FormEvent, ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { Button, Cross, EmailInput, Input, UnoLogo } from './ui'
import { LEARNER_42_EMAIL_HINT, isLearner42Email } from '../validation'
import { requestErrorMessage } from '../utils/errors'

export type AuthMode = 'login' | 'register'

type AuthModalContextValue = {
  openAuth: (mode?: AuthMode, destination?: string) => void
  closeAuth: () => void
}

const AuthModalContext = createContext<AuthModalContextValue | undefined>(undefined)

export function AuthModalProvider({ children }: { children: ReactNode }) {
  const [request, setRequest] = useState<{ mode: AuthMode; destination?: string } | null>(null)
  const openAuth = useCallback((mode: AuthMode = 'login', destination?: string) => setRequest({ mode, destination }), [])
  const closeAuth = useCallback(() => setRequest(null), [])
  const contextValue = useMemo(() => ({ openAuth, closeAuth }), [closeAuth, openAuth])

  return (
    <AuthModalContext.Provider value={contextValue}>
      {children}
      {request ? <AuthModal mode={request.mode} destination={request.destination} /> : null}
    </AuthModalContext.Provider>
  )
}

export function useAuthModal() {
  const context = useContext(AuthModalContext)
  if (!context) throw new Error('useAuthModal must be used inside AuthModalProvider')
  return context
}

export function AuthButton({ mode = 'login', className = '', onClick, children, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { mode?: AuthMode }) {
  const { openAuth } = useAuthModal()

  return (
    <button
      {...props}
      type="button"
      className={`button ${mode === 'login' ? 'button-secondary' : 'button-primary'} ${className}`}
      onClick={(event) => {
        onClick?.(event)
        if (!event.defaultPrevented) openAuth(mode)
      }}
    >
      {children ?? (mode === 'login' ? 'Sign in' : 'Create account')}
    </button>
  )
}

function AuthModal({ mode: initialMode, destination }: { mode: AuthMode; destination?: string }) {
  const { signIn } = useAuth()
  const { closeAuth } = useAuthModal()
  const navigate = useNavigate()
  const [mode, setMode] = useState(initialMode)
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [acceptedTerms, setAcceptedTerms] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !busy) closeAuth()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [busy, closeAuth])

  const switchMode = (nextMode: AuthMode) => {
    setMode(nextMode)
    setError('')
    setPassword('')
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setBusy(true)
    setError('')

    if (!isLearner42Email(email)) {
      setError(LEARNER_42_EMAIL_HINT)
      setBusy(false)
      return
    }
    if (mode === 'register' && !acceptedTerms) {
      setError('Accept the terms to continue.')
      setBusy(false)
      return
    }

    try {
      const response = mode === 'login'
        ? await api.login(email, password)
        : await api.register(username, email, password)
      signIn(response)
      closeAuth()
      navigate(destination ?? '/dashboard', { replace: true })
    } catch (requestError) {
      setError(requestErrorMessage(requestError, 'Something went wrong. Please try again.'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth-modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) closeAuth() }}>
      <section className="auth-modal" role="dialog" aria-modal="true" aria-labelledby="auth-modal-title">
        <div className="auth-modal-heading">
          <div>
            <UnoLogo />
            <h2 id="auth-modal-title">{mode === 'login' ? 'Sign in' : 'Create account'}</h2>
            <p>{mode === 'login' ? 'Continue to play.' : 'Join Uno Ranked.'}</p>
          </div>
          <button className="dialog-close" type="button" onClick={closeAuth} disabled={busy} aria-label="Close"><Cross className="ui-icon" size={17} aria-hidden="true" /></button>
        </div>
        <form className="stack-form" onSubmit={submit} noValidate>
          {error ? <div className="form-alert" role="alert">{error}</div> : null}
          {mode === 'register' ? <Input id="modal-register-username" label="Username" hint="3–24 letters, numbers or underscores." autoComplete="username" value={username} onChange={(event) => setUsername(event.target.value)} minLength={3} maxLength={24} required /> : null}
          <EmailInput id={`modal-${mode}-email`} label="Email" value={email} onChange={setEmail} hint={LEARNER_42_EMAIL_HINT} required />
          <Input id={`modal-${mode}-password`} label="Password" type="password" hint={mode === 'register' ? 'Use at least 12 characters.' : undefined} autoComplete={mode === 'login' ? 'current-password' : 'new-password'} value={password} onChange={(event) => setPassword(event.target.value)} minLength={mode === 'register' ? 12 : undefined} required />
          {mode === 'register' ? <label className="checkbox-field"><input type="checkbox" checked={acceptedTerms} onChange={(event) => setAcceptedTerms(event.target.checked)} /><span>I agree to the <a href="/terms" target="_blank" rel="noreferrer">Terms</a> and <a href="/privacy" target="_blank" rel="noreferrer">Privacy Policy</a>.</span></label> : null}
          <Button type="submit" disabled={busy}>{busy ? (mode === 'login' ? 'Signing in…' : 'Creating account…') : (mode === 'login' ? 'Sign in' : 'Create account')}</Button>
        </form>
        <div className="auth-modal-footer">
          <span>{mode === 'login' ? 'New here?' : 'Already have an account?'}</span>
          <button className="text-button" type="button" onClick={() => switchMode(mode === 'login' ? 'register' : 'login')} disabled={busy}>{mode === 'login' ? 'Create account' : 'Sign in'}</button>
        </div>
      </section>
    </div>
  )
}
