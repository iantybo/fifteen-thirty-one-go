import React, { createContext, useContext, useMemo, useState } from 'react'
import type { User } from '../api/types'

type AuthState = {
  token: string | null
  user: User | null
  setAuth: (token: string, user: User) => void
  clearAuth: () => void
}

const AuthContext = createContext<AuthState | undefined>(undefined)

const LS_TOKEN = 'fto_token'
const LS_USER = 'fto_user'

function isUser(v: unknown): v is User {
  if (!v || typeof v !== 'object') return false
  const o = v as any
  return typeof o.id === 'number' && Number.isFinite(o.id) && typeof o.username === 'string'
}

function loadInitial(): { token: string | null; user: User | null } {
  const token = localStorage.getItem(LS_TOKEN)
  const userRaw = localStorage.getItem(LS_USER)
  if (!token || !userRaw) return { token: null, user: null }
  try {
    const parsed = JSON.parse(userRaw) as unknown
    if (!isUser(parsed)) {
      localStorage.removeItem(LS_TOKEN)
      localStorage.removeItem(LS_USER)
      return { token: null, user: null }
    }
    return { token, user: parsed }
  } catch {
    localStorage.removeItem(LS_TOKEN)
    localStorage.removeItem(LS_USER)
    return { token: null, user: null }
  }
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const init = useMemo(loadInitial, [])
  const [token, setToken] = useState<string | null>(init.token)
  const [user, setUser] = useState<User | null>(init.user)

  const value = useMemo<AuthState>(
    () => ({
      token,
      user,
      setAuth: (t, u) => {
        setToken(t)
        setUser(u)
        localStorage.setItem(LS_TOKEN, t)
        localStorage.setItem(LS_USER, JSON.stringify(u))
      },
      clearAuth: () => {
        setToken(null)
        setUser(null)
        localStorage.removeItem(LS_TOKEN)
        localStorage.removeItem(LS_USER)
      },
    }),
    [token, user],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}


