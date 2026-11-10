import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { api } from '../api/client'
import { realtime } from '../realtime/client'
import type { AuthResponse, User } from '../types'

const TOKEN_KEY = 'card-arena.access-token'
const DEBUG_AUTH = import.meta.env.VITE_DEBUG_AUTH === 'true'
const DEBUG_TOKEN = 'debug'

type AuthContextValue = {
  user: User | null
  loading: boolean
  signIn: (response: AuthResponse) => void
  signOut: () => Promise<void>
  refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

function applyToken(token: string | null) {
  if (token) sessionStorage.setItem(TOKEN_KEY, token)
  else sessionStorage.removeItem(TOKEN_KEY)
  api.setToken(token)
  realtime.setToken(token)
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const storedToken = sessionStorage.getItem(TOKEN_KEY)
    const staleDebugToken = !DEBUG_AUTH && storedToken === DEBUG_TOKEN
    const token = DEBUG_AUTH ? DEBUG_TOKEN : staleDebugToken ? null : storedToken
    applyToken(token)
    if (!token) {
      setLoading(false)
      return
    }

    api.me()
      .then(setUser)
      .catch(() => {
        applyToken(null)
        setUser(null)
      })
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => realtime.onUnauthorized(() => {
    applyToken(null)
    setUser(null)
  }), [])

  const value = useMemo<AuthContextValue>(() => ({
    user,
    loading,
    signIn: (response) => {
      applyToken(response.accessToken)
      setUser(response.user)
    },
    signOut: async () => {
      try {
        await api.logout()
      } finally {
        applyToken(null)
        setUser(null)
      }
    },
    refreshUser: async () => {
      setUser(await api.me())
    },
  }), [loading, user])

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) throw new Error('useAuth must be used inside AuthProvider')
  return context
}
