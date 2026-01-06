import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import type { Lobby } from '../api/types'
import { useAuth } from '../auth/auth'
import { usePresence } from '../hooks/usePresence'

export function LobbiesPage() {
  const { user, clearAuth } = useAuth()
  const nav = useNavigate()
  const [lobbies, setLobbies] = useState<Lobby[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [quickBusy, setQuickBusy] = useState(false)
  const { onlineUsers, connected } = usePresence()

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
    <div style={{
      minHeight: '100vh',
      background: 'var(--color-lobby-bg)',
      padding: '24px 16px'
    }}>
      <div style={{
        maxWidth: 800,
        margin: '0 auto',
        background: 'var(--color-lobby-card)',
        borderRadius: '16px',
        padding: '24px',
        boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04)'
      }}>
        <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: '24px', flexWrap: 'wrap', gap: '12px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <h1 style={{ margin: 0 }}>Lobbies</h1>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '14px', color: connected ? 'var(--color-online)' : 'var(--color-offline)' }}>
              <span style={{
                width: '8px',
                height: '8px',
                borderRadius: '50%',
                background: connected ? 'var(--color-online)' : 'var(--color-offline)',
                display: 'inline-block'
              }} />
              {connected ? 'Connected' : 'Disconnected'}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 12, alignItems: 'baseline' }}>
            <Link to="/lobbies/new">Create</Link>
            <Link to="/leaderboard">Leaderboard</Link>
            <button onClick={playVsBot} disabled={quickBusy} title="Create a 2-player lobby and add a bot">
              {quickBusy ? 'Starting…' : 'Play vs Computer'}
            </button>
            <button onClick={clearAuth}>Logout</button>
          </div>
        </header>

        {onlineUsers.length > 0 && (
          <div style={{
            marginBottom: '24px',
            padding: '16px',
            background: '#ffffff',
            borderRadius: '8px',
            boxShadow: '0 1px 3px 0 rgba(0, 0, 0, 0.1)'
          }}>
            <h3 style={{ margin: '0 0 12px 0', fontSize: '14px', fontWeight: '600', color: '#64748b' }}>
              Online Players ({onlineUsers.filter(u => u.status !== 'offline').length})
            </h3>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
              {onlineUsers.filter(u => u.status !== 'offline').map((presence) => (
                <div
                  key={presence.user_id}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '6px',
                    padding: '6px 12px',
                    background: '#f8fafc',
                    borderRadius: '6px',
                    fontSize: '14px'
                  }}
                >
                  <span style={{
                    width: '8px',
                    height: '8px',
                    borderRadius: '50%',
                    background: presence.status === 'online' ? 'var(--color-online)' :
                               presence.status === 'away' ? 'var(--color-away)' :
                               presence.status === 'in_game' ? 'var(--color-in-game)' :
                               'var(--color-offline)',
                    display: 'inline-block'
                  }} />
                  <span>{presence.username}</span>
                  {presence.status === 'in_game' && <span style={{ fontSize: '12px', opacity: 0.7 }}>(in game)</span>}
                </div>
              ))}
            </div>
          </div>
        )}

        {err && <div style={{ color: 'crimson', marginBottom: '16px' }}>{err}</div>}
        {loading && <div>Loading lobbies...</div>}
        {!loading && !err && lobbies.length === 0 && (
          <div style={{ marginTop: 12, opacity: 0.8 }}>
            No lobbies yet. <Link to="/lobbies/new">Create one</Link>.
          </div>
        )}
        <ul style={{ listStyle: 'none', padding: 0 }}>
          {lobbies.map((l) => (
            <li key={l.id} style={{
              margin: '12px 0',
              padding: '16px',
              background: '#ffffff',
              borderRadius: '8px',
              boxShadow: '0 1px 3px 0 rgba(0, 0, 0, 0.1)',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center'
            }}>
              <div>
                <b>{l.name}</b> — {l.current_players}/{l.max_players} — {l.status}
              </div>
              <Link to={`/lobbies/${l.id}`} style={{ padding: '8px 16px', background: 'var(--color-primary)', color: 'white', borderRadius: '6px', textDecoration: 'none' }}>Open</Link>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}


