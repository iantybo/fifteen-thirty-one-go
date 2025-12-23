import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/auth'

export function RegisterPage() {
  const { setAuth } = useAuth()
  const nav = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!username.trim() || !password.trim()) {
      setErr('Username and password are required')
      return
    }
    setErr(null)
    setBusy(true)
    try {
      const res = await api.register({ username, password })
      setAuth(res.token, res.user)
      nav('/lobbies', { replace: true })
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : 'register failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{ maxWidth: 420, margin: '40px auto' }}>
      <h1>Create account</h1>
      <form onSubmit={onSubmit}>
        <label>
          Username
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
        </label>
        <label>
          Password
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" />
        </label>
        {err && <div style={{ color: 'crimson', marginTop: 8 }}>{err}</div>}
        <button disabled={busy} style={{ marginTop: 12 }}>
          {busy ? 'Creating…' : 'Create'}
        </button>
      </form>
      <p style={{ marginTop: 16 }}>
        Already have an account? <Link to="/login">Sign in</Link>
      </p>
    </div>
  )
}


