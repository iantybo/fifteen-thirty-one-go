import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import type { Lobby } from '../api/types'
import { useAuth } from '../auth/auth'

export function LobbiesPage() {
  const { user, clearAuth } = useAuth()
  const nav = useNavigate()
  const [lobbies, setLobbies] = useState<Lobby[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [quickBusy, setQuickBusy] = useState(false)

  useEffect(() => {
    let cancelled = false
    async function load() {
      if (!user) return
      setErr(null)
      setLoading(true)
      try {
        const res = await api.listLobbies()
        if (!cancelled) setLobbies(res.lobbies)
      } catch (e: unknown) {
        if (!cancelled) setErr(e instanceof Error ? e.message : 'failed to load lobbies')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [user])

  async function playVsBot() {
    if (!user) {
      setErr('You must be logged in')
      return
    }
    setErr(null)
    setQuickBusy(true)
    try {
      const created = await api.createLobby({ name: 'Vs Computer', max_players: 2 })
      await api.addBotToLobby(created.lobby.id, { difficulty: 'easy' })
      nav(`/games/${created.game.id}`, { replace: true })
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : 'failed to start vs bot')
    } finally {
      setQuickBusy(false)
    }
  }

  return (
    <div style={{ maxWidth: 800, margin: '24px auto', padding: '0 16px' }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
        <h1>Lobbies</h1>
        <div style={{ display: 'flex', gap: 12, alignItems: 'baseline' }}>
          <Link to="/lobbies/new">Create</Link>
          <button onClick={playVsBot} disabled={quickBusy} title="Create a 2-player lobby and add a bot">
            {quickBusy ? 'Starting…' : 'Play vs Computer'}
          </button>
          <button onClick={clearAuth}>Logout</button>
        </div>
      </header>

      {err && <div style={{ color: 'crimson' }}>{err}</div>}
      {loading && <div>Loading lobbies...</div>}
      {!loading && !err && lobbies.length === 0 && (
        <div style={{ marginTop: 12, opacity: 0.8 }}>
          No lobbies yet. <Link to="/lobbies/new">Create one</Link>.
        </div>
      )}
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


