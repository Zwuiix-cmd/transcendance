import { lazy, Suspense, useEffect } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { AuthModalProvider, useAuthModal, type AuthMode } from './components/AuthModal'
import { ProtectedRoute } from './components/ProtectedRoute'
import { MatchmakingProvider, useMatchmaking } from './components/MatchmakingLauncher'
import { LoadingState } from './components/ui'

const DashboardPage = lazy(() => import('./pages/DashboardPage').then((module) => ({ default: module.DashboardPage })))
const GamesPage = lazy(() => import('./pages/GamesPage').then((module) => ({ default: module.GamesPage })))
const GameLobbyPage = lazy(() => import('./pages/GameLobbyPage').then((module) => ({ default: module.GameLobbyPage })))
const LeaderboardPage = lazy(() => import('./pages/LeaderboardPage').then((module) => ({ default: module.LeaderboardPage })))
const LegalPage = lazy(() => import('./pages/LegalPages').then((module) => ({ default: module.LegalPage })))
const ProfilePage = lazy(() => import('./pages/ProfilePage').then((module) => ({ default: module.ProfilePage })))
const AccountPage = lazy(() => import('./pages/AccountPage').then((module) => ({ default: module.AccountPage })))
const PublicProfilePage = lazy(() => import('./pages/PublicProfilePage').then((module) => ({ default: module.PublicProfilePage })))
const UnoGamePage = lazy(() => import('./pages/UnoGamePage').then((module) => ({ default: module.UnoGamePage })))
const UnoResultsPage = lazy(() => import('./pages/UnoResultsPage').then((module) => ({ default: module.UnoResultsPage })))

function RouteLoading() {
  return <main className="main-content route-loading"><LoadingState label="Loading page" /></main>
}

function HomeRedirect() {
  return <Navigate to="/dashboard" replace />
}

function AuthRedirect({ mode }: { mode: AuthMode }) {
  const location = useLocation()
  const { openAuth } = useAuthModal()

  useEffect(() => {
    const state = location.state as { from?: string } | null
    openAuth(mode, state?.from ?? '/dashboard')
  }, [location.state, mode, openAuth])

  return <Navigate to="/dashboard" replace />
}

function PlayRedirect() {
  const { openLauncher } = useMatchmaking()
  useEffect(() => openLauncher(), [openLauncher])
  return <Navigate to="/dashboard" replace />
}

export default function App() {
  return <Suspense fallback={<RouteLoading />}>
    <Routes>
      <Route path="/" element={<HomeRedirect />} />
      <Route path="/:kind" element={<LegalPage />} />
      <Route path="/uno" element={<UnoGamePage />} />
      <Route path="/uno/results" element={<UnoResultsPage />} />
      <Route element={<AuthModalProvider><MatchmakingProvider><AppShell /></MatchmakingProvider></AuthModalProvider>}>
        <Route path="/login" element={<AuthRedirect mode="login" />} />
        <Route path="/register" element={<AuthRedirect mode="register" />} />
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/leaderboard" element={<LeaderboardPage />} />
        <Route path="/players/:id" element={<PublicProfilePage />} />
        <Route element={<ProtectedRoute />}>
          <Route path="/play" element={<PlayRedirect />} />
          <Route path="/games" element={<GamesPage />} />
          <Route path="/games/:id" element={<GameLobbyPage />} />
          <Route path="/profile" element={<ProfilePage />} />
          <Route path="/account" element={<AccountPage />} />
        </Route>
      </Route>
      <Route path="*" element={<HomeRedirect />} />
    </Routes>
  </Suspense>
}
