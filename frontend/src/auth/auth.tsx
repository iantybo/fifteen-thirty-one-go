import React, { createContext, useContext, useEffect, useMemo, useState } from 'react'
import { api } from '../api/client'
import type { User } from '../api/types'

type AuthState = {
  user: User | null
  loading: boolean
  setAuth: (user: User) => void
  clearAuth: () => void
}

const AuthContext = createContext<AuthState | undefined>(undefined)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    async function loadSession() {
      try {
        const res = await api.me()
        if (!cancelled) setUser(res.user)
      } catch {
        // Not logged in (or session invalid). Ignore.
        if (!cancelled) setUser(null)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void loadSession()
    return () => {
      cancelled = true
    }
  }, [])

  const value = useMemo<AuthState>(
    () => ({
      user,
      loading,
      setAuth: (u) => {
        setUser(u)
      },
      clearAuth: () => {
        setUser(null)
        // Best-effort: clear server cookie session too (logout() never rejects).
        void api.logout()
      },
    }),
    [user, loading],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}


