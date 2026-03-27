import React, { createContext, useContext, useEffect, useMemo, useState } from 'react'
import { api } from '../api/client'
import type { User } from '../api/types'

type AuthState = {
  user: User | null
  loading: boolean
  /** True when the current session was established via wallet (SIWE-style) sign-in. */
  authViaWallet: boolean
  setAuth: (user: User, opts?: { viaWallet?: boolean }) => void
  clearAuth: () => void
}

const AuthContext = createContext<AuthState | undefined>(undefined)

/** Persists whether this tab signed in via wallet (not inferred from user.wallet_address). */
const AUTH_VIA_WALLET_KEY = 'fto_auth_via_wallet'

function readAuthViaWalletFlag(): boolean {
  if (typeof sessionStorage === 'undefined') return false
  return sessionStorage.getItem(AUTH_VIA_WALLET_KEY) === '1'
}

function writeAuthViaWalletFlag(viaWallet: boolean) {
  if (typeof sessionStorage === 'undefined') return
  if (viaWallet) sessionStorage.setItem(AUTH_VIA_WALLET_KEY, '1')
  else sessionStorage.removeItem(AUTH_VIA_WALLET_KEY)
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [authViaWallet, setAuthViaWallet] = useState(false)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    async function loadSession() {
      try {
        const res = await api.me()
        if (!cancelled) {
          setUser(res.user)
          setAuthViaWallet(readAuthViaWalletFlag())
        }
      } catch {
        // Not logged in (or session invalid). Ignore.
        if (!cancelled) {
          setUser(null)
          setAuthViaWallet(false)
          writeAuthViaWalletFlag(false)
        }
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
      authViaWallet,
      setAuth: (u, opts) => {
        setUser(u)
        const via = Boolean(opts?.viaWallet)
        setAuthViaWallet(via)
        writeAuthViaWalletFlag(via)
      },
      clearAuth: () => {
        setUser(null)
        setAuthViaWallet(false)
        writeAuthViaWalletFlag(false)
        // Best-effort: clear server cookie session too (logout() never rejects).
        void api.logout()
      },
    }),
    [user, loading, authViaWallet],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}


