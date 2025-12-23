import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/auth'

export function LoginPage() {
  const { setAuth } = useAuth()
  const nav = useNavigate()
  const loc = useLocation() as any
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setErr(null)
    setBusy(true)
    try {
      const res = await api.login({ username, password })
      setAuth(res.token, res.user)
      nav(loc?.state?.from ?? '/lobbies', { replace: true })
    } catch (e: any) {
      setErr(e?.message ?? 'login failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{ maxWidth: 420, margin: '40px auto' }}>
      <h1>Login</h1>
      <form onSubmit={onSubmit}>
        <label>
          Username
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
        </label>
        <label>
          Password
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" />
        </label>
        {err && <div style={{ color: 'crimson', marginTop: 8 }}>{err}</div>}
        <button disabled={busy} style={{ marginTop: 12 }}>
          {busy ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
      <p style={{ marginTop: 16 }}>
        New here? <Link to="/register">Create an account</Link>
      </p>
    </div>
  )
}


