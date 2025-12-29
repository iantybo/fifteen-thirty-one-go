import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/auth'

export function CreateLobbyPage() {
  const { user } = useAuth()
  const nav = useNavigate()
  const [name, setName] = useState('Lobby')
  const [maxPlayers, setMaxPlayers] = useState(2)
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!user) {
      setErr('You must be logged in to create a lobby')
      return
    }
    setErr(null)
    const trimmed = name.trim()
    if (trimmed === '') {
      setErr('Lobby name is required')
      return
    }
    setBusy(true)
    try {
      const res = await api.createLobby({ name: trimmed, max_players: maxPlayers })
      nav(`/games/${res.game.id}`, { replace: true })
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : 'failed to create lobby')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{ maxWidth: 500, margin: '24px auto', padding: '0 16px' }}>
      <h1>Create lobby</h1>
      <form onSubmit={onSubmit}>
        <label htmlFor="lobby_name">Name</label>
        <input
          id="lobby_name"
          value={name}
          onChange={(e) => {
            setName(e.target.value)
            if (err) setErr(null)
          }}
          required
          maxLength={100}
        />
        <label htmlFor="lobby_max_players">Max players</label>
        <select
          id="lobby_max_players"
          value={maxPlayers}
          onChange={(e) => setMaxPlayers(Number(e.target.value))}
        >
            <option value={2}>2</option>
            <option value={3}>3</option>
            <option value={4}>4</option>
        </select>
        {err && <div style={{ color: 'crimson', marginTop: 8 }}>{err}</div>}
        <button disabled={busy} style={{ marginTop: 12 }}>
          {busy ? 'Creating…' : 'Create'}
        </button>
      </form>
    </div>
  )
}


