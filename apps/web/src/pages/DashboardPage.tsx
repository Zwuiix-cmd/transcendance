import { Link } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { AuthButton } from '../components/AuthModal'
import { PlayButton } from '../components/MatchmakingLauncher'
import { ArrowRight, Card } from '../components/ui'

export function DashboardPage() {
  const { user } = useAuth()

  if (!user) return <div className="page-stack home-page">
    <section className="home-hero home-hero-centered home-hero-public">
      <div className="home-hero-copy"><span className="eyebrow">Uno Ranked</span><h1>A calmer way to play Uno.</h1><p>Find your place on the leaderboard, then join the table when you are ready.</p><div className="home-hero-actions"><AuthButton mode="register" className="home-create-account-button">Create account</AuthButton><Link className="text-link home-leaderboard-link" to="/leaderboard">Leaderboard</Link></div></div>
    </section>
    <div className="home-public-grid">
      <Card className="home-public-card"><span className="eyebrow">Ranked</span><h2>Build your rating.</h2><p>Start at 1000 ELO and find opponents close to your level.</p><Link className="text-link" to="/leaderboard">View the leaderboard <ArrowRight className="ui-icon" size={15} aria-hidden="true" /></Link></Card>
      <Card className="home-public-card"><span className="eyebrow">Unranked</span><h2>Play without pressure.</h2><p>Practice and play with no rating change when you just want a game.</p><span className="muted home-public-note">Available after sign in</span></Card>
      <Card className="home-public-card"><span className="eyebrow">Profiles</span><h2>Follow every climb.</h2><p>Open a player profile to see their ELO, record and long-term stats.</p><Link className="text-link" to="/leaderboard">Find a player <ArrowRight className="ui-icon" size={15} aria-hidden="true" /></Link></Card>
    </div>
  </div>
  const stats = user.stats
  const winRate = stats.gamesPlayed === 0 ? 0 : Math.round((stats.gamesWon / stats.gamesPlayed) * 100)

  return <div className="page-stack home-page">
    <section className="home-hero home-hero-centered">
      <div className="home-hero-copy"><h1>Play next.</h1><p>Choose a mode and start your next game.</p><div className="home-hero-actions"><PlayButton>Play</PlayButton><Link className="button button-secondary" to="/leaderboard">Leaderboard</Link></div></div>
    </section>
    <div className="home-summary-grid">
      <Card className="home-rating-card"><div><span className="eyebrow">Current rating</span><strong>{user.elo}</strong><small>{user.elo === 1000 ? 'Starting rating' : 'ELO points'}</small></div><Link className="text-link" to="/profile">View profile <ArrowRight className="ui-icon" size={15} aria-hidden="true" /></Link></Card>
      <Card className="home-record-card"><div className="section-heading"><div><span className="eyebrow">Record</span><h2>Your games</h2></div><span className="muted">{stats.currentStreak} streak</span></div><div className="home-record-grid"><div><strong>{stats.gamesWon}</strong><span>Wins</span></div><div><strong>{stats.gamesPlayed}</strong><span>Games</span></div><div><strong>{winRate}%</strong><span>Win rate</span></div></div></Card>
    </div>
  </div>
}
