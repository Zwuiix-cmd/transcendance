import { useEffect, useRef, useState } from 'react'
import { NavLink, Outlet, Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { AuthButton } from './AuthModal'
import { MatchmakingLauncher, PlayButton } from './MatchmakingLauncher'
import { useRealtimeStatus } from '../realtime/hooks'
import { Button, ChevronDown, Logout, UnoLogo, UserAvatar } from './ui'

function RealtimeBanner() {
  const status = useRealtimeStatus()
  if (status !== 'reconnecting' && status !== 'offline') return null
  return <div className="realtime-banner" role="status">
    <span className="realtime-banner-dot" aria-hidden="true" />
    {status === 'offline' ? 'You are offline. Live updates resume when the connection is back.' : 'Reconnecting to live updates…'}
  </div>
}

export function AppShell() {
  const { user, signOut } = useAuth()
  const navigate = useNavigate()
  const menuRef = useRef<HTMLDivElement>(null)
  const [menuOpen, setMenuOpen] = useState(false)

  useEffect(() => {
    const closeOnOutsideClick = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) setMenuOpen(false)
    }
    document.addEventListener('mousedown', closeOnOutsideClick)
    return () => document.removeEventListener('mousedown', closeOnOutsideClick)
  }, [])

  const logout = async () => {
    setMenuOpen(false)
    await signOut()
    navigate('/dashboard')
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <Link className="brand" to="/dashboard" aria-label="Uno Ranked home">
          <UnoLogo />
          <span>Uno <em>Ranked</em></span>
        </Link>
        <nav className="main-nav" aria-label="Primary navigation">
          <NavLink to="/leaderboard">Leaderboard</NavLink>
        </nav>
        <div className="topbar-actions">
          {user ? <>
            <PlayButton className="topbar-play-button">Play</PlayButton>
            <div className="topbar-user" ref={menuRef}>
              <button
              className="profile-menu-trigger"
              type="button"
              aria-haspopup="menu"
              aria-expanded={menuOpen}
              onClick={() => setMenuOpen((open) => !open)}
              >
                <UserAvatar username={user.username} avatarUrl={user.avatarUrl} />
                <span className="profile-menu-copy"><strong>{user.username}</strong></span>
                <ChevronDown className="profile-menu-chevron ui-icon" size={16} aria-hidden="true" />
              </button>
              {menuOpen ? <div className="profile-menu" role="menu">
                <Link role="menuitem" to="/profile" onClick={() => setMenuOpen(false)}>Profile</Link>
                <Link role="menuitem" to="/account" onClick={() => setMenuOpen(false)}>Account</Link>
                {user.role !== 'player' ? <a role="menuitem" className="admin-menu-item" href="/admin/" onClick={() => setMenuOpen(false)}><span>Admin panel</span></a> : null}
                <div className="profile-menu-divider" />
                <Button role="menuitem" variant="ghost" className="profile-menu-logout" onClick={() => void logout()}><Logout className="ui-icon" size={16} aria-hidden="true" />Log out</Button>
              </div> : null}
            </div>
          </> : <div className="topbar-auth-actions">
            <AuthButton mode="login" className="topbar-auth-login">Sign in</AuthButton>
            <AuthButton mode="register" className="topbar-auth-register">Create account</AuthButton>
          </div>}
        </div>
      </header>
      <RealtimeBanner />
      <main className="main-content"><Outlet /></main>
      <footer className="footer">
        <div><strong>Uno Ranked</strong><span>Competitive games, measured fairly.</span></div>
        <span className="footer-links"><Link to="/privacy">Privacy</Link><Link to="/terms">Terms</Link><Link to="/leaderboard">Leaderboard</Link></span>
      </footer>
      <MatchmakingLauncher />
    </div>
  )
}
