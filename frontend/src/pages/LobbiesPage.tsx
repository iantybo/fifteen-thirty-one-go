import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import type { Lobby } from '../api/types'
import { useAuth } from '../auth/auth'

export function LobbiesPage() {
  const { token, clearAuth } = useAuth()
  const [lobbies, setLobbies] = useState<Lobby[]>([])
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    async function load() {
      if (!token) return
      setErr(null)
      try {
        const res = await api.listLobbies(token)
        if (!cancelled) setLobbies(res.lobbies)
      } catch (e: any) {
        if (!cancelled) setErr(e?.message ?? 'failed to load lobbies')
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [token])

  return (
    <div style={{ maxWidth: 800, margin: '24px auto', padding: '0 16px' }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
        <h1>Lobbies</h1>
        <div style={{ display: 'flex', gap: 12, alignItems: 'baseline' }}>
          <Link to="/lobbies/new">Create</Link>
          <button onClick={clearAuth}>Logout</button>
        </div>
      </header>

      {err && <div style={{ color: 'crimson' }}>{err}</div>}
      <ul>
        {lobbies.map((l) => (
          <li key={l.id} style={{ margin: '10px 0' }}>
            <b>{l.name}</b> — {l.current_players}/{l.max_players} — {l.status}{' '}
            <Link to={`/lobbies/${l.id}`}>Open</Link>
          </li>
        ))}
      </ul>
    </div>
  )
}


