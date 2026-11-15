import { useEffect, useState } from 'react'
import type { ButtonHTMLAttributes, HTMLAttributes, InputHTMLAttributes, ReactNode, TextareaHTMLAttributes } from 'react'
import { ArrowLeft, ArrowRight, Camera, Card as CardIcon, ChevronDown, ChevronRight, Cross, Crown, Globe, Link as LinkIcon, Logout, Play, Search, Sparkles } from '@getpapillon/papicons'

export { ArrowLeft, ArrowRight, Camera, CardIcon, ChevronDown, ChevronRight, Cross, Crown, Globe, LinkIcon, Logout, Play, Search, Sparkles }

export function Button({
  variant = 'primary',
  className = '',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'secondary' | 'ghost' | 'danger' }) {
  return <button className={`button button-${variant} ${className}`} {...props} />
}

export function UnoLogo() {
  return (
    <span className="uno-logo" aria-hidden="true">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" role="img">
        <rect x="3" y="3" width="26" height="26" rx="8" fill="#a78bfa" transform="rotate(-8 16 16)" />
        <ellipse cx="16" cy="16" rx="11" ry="6" fill="#eee9fa" transform="rotate(-8 16 16)" />
        <text x="16" y="18" textAnchor="middle" fill="#292332" fontSize="6.5" fontWeight="800" fontFamily="Arial, sans-serif">UNO</text>
      </svg>
    </span>
  )
}

export function UserAvatar({
  username,
  avatarUrl,
  className = '',
}: {
  username: string
  avatarUrl?: string | null
  className?: string
}) {
  const [imageFailed, setImageFailed] = useState(false)
  useEffect(() => setImageFailed(false), [avatarUrl])
  const size = className.includes('large') ? 76 : className.includes('small') ? 30 : 34
  return (
    <span className={`avatar ${className}`} aria-label={`${username} avatar`}>
      {avatarUrl && !imageFailed ? <img src={avatarUrl} alt="" width={size} height={size} loading="lazy" decoding="async" onError={() => setImageFailed(true)} /> : username.slice(0, 1).toUpperCase()}
    </span>
  )
}

export function EmailInput({
  label,
  hint,
  error,
  id,
  value,
  onChange,
  required = false,
}: {
  label: string
  hint?: string
  error?: string
  id: string
  value: string
  onChange: (email: string) => void
  required?: boolean
}) {
  const localPart = value.split('@', 1)[0]
  const hintId = `${id}-hint`

  return (
    <label className="field" htmlFor={id}>
      <span className="field-label">{label}</span>
      <span className={`email-input-shell ${error ? 'input-error' : ''}`}>
        <input
          id={id}
          className="input email-local-input"
          type="text"
          inputMode="email"
          autoComplete="email"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
          value={localPart}
          onChange={(event) => {
            const nextLocalPart = event.target.value.trim().split('@', 1)[0].replace(/\s/g, '')
            onChange(nextLocalPart ? `${nextLocalPart}@learner.42.tech` : '')
          }}
          aria-describedby={hint ? hintId : undefined}
          required={required}
          title="Enter the part before @learner.42.tech"
        />
        <span className="email-domain" aria-hidden="true">@learner.42.tech</span>
      </span>
      {error ? <span className="field-error" role="alert">{error}</span> : hint ? <span id={hintId} className="field-hint">{hint}</span> : null}
    </label>
  )
}

export function Input({ label, hint, error, id, ...props }: InputHTMLAttributes<HTMLInputElement> & {
  label: string
  hint?: string
  error?: string
}) {
  return (
    <label className="field" htmlFor={id}>
      <span className="field-label">{label}</span>
      <input id={id} className={`input ${error ? 'input-error' : ''}`} {...props} />
      {error ? <span className="field-error" role="alert">{error}</span> : hint ? <span className="field-hint">{hint}</span> : null}
    </label>
  )
}

export function TextArea({ label, hint, error, id, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement> & {
  label: string
  hint?: string
  error?: string
}) {
  return (
    <label className="field" htmlFor={id}>
      <span className="field-label">{label}</span>
      <textarea id={id} className={`input textarea ${error ? 'input-error' : ''}`} {...props} />
      {error ? <span className="field-error" role="alert">{error}</span> : hint ? <span className="field-hint">{hint}</span> : null}
    </label>
  )
}

export function Card({ children, className = '', ...props }: HTMLAttributes<HTMLElement>) {
  return <section {...props} className={`card ${className}`}>{children}</section>
}

export function Badge({ children, tone = 'neutral' }: { children: ReactNode; tone?: 'neutral' | 'success' | 'warning' | 'danger' }) {
  return <span className={`badge badge-${tone}`}>{children}</span>
}

export function StatCard({ label, value, detail }: { label: string; value: ReactNode; detail?: string }) {
  return (
    <Card className="stat-card">
      <span className="eyebrow">{label}</span>
      <strong className="stat-value">{value}</strong>
      {detail ? <span className="muted">{detail}</span> : null}
    </Card>
  )
}

export function PageHeader({ eyebrow, title, description, action }: {
  eyebrow?: string
  title: string
  description?: string
  action?: ReactNode
}) {
  return (
    <div className="page-header">
      <div>
        {eyebrow ? <span className="eyebrow">{eyebrow}</span> : null}
        <h1>{title}</h1>
        {description ? <p className="page-description">{description}</p> : null}
      </div>
      {action ? <div className="page-header-action">{action}</div> : null}
    </div>
  )
}

export function EmptyState({ title, description }: { title: string; description: string }) {
  return (
    <div className="empty-state">
      <div className="empty-icon" aria-hidden="true"><Sparkles className="ui-icon" size={22} /></div>
      <h2>{title}</h2>
      <p className="muted">{description}</p>
    </div>
  )
}

export function LoadingState({ label = 'Loading' }: { label?: string }) {
  return <div className="loading-state" role="status" aria-label={label}>
    <span className="loading-skeleton loading-skeleton-wide" aria-hidden="true" />
    <span className="loading-skeleton loading-skeleton-medium" aria-hidden="true" />
    <span className="loading-skeleton loading-skeleton-short" aria-hidden="true" />
  </div>
}
