import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/auth'

export function LobbyDetailPage() {
  const { id } = useParams()
  const lobbyId = Number(id)
  const isValidId = Number.isFinite(lobbyId) && lobbyId > 0
  const { user } = useAuth()
  const nav = useNavigate()
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const canJoin = !!user && isValidId && !busy

  async function join() {
    if (!user) {
      setErr('You must be logged in to join a lobby')
      return
    }
    if (!isValidId) {
      setErr('Invalid lobby id')
      return
    }
    setErr(null)
    setBusy(true)
    try {
      const res = await api.joinLobby(lobbyId)
      nav(`/games/${res.game_id}`, { replace: true })
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : 'failed to join')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{ maxWidth: 700, margin: '24px auto', padding: '0 16px' }}>
      <h1>Lobby {isValidId ? lobbyId : 'Invalid ID'}</h1>
      <button
        disabled={!canJoin}
        aria-disabled={!canJoin}
        title={!user ? 'Log in to join this lobby' : !isValidId ? 'Invalid lobby id' : undefined}
        onClick={join}
      >
        {busy ? 'Joining…' : 'Join lobby'}
      </button>
      {err && <div style={{ color: 'crimson', marginTop: 8 }}>{err}</div>}
    </div>
  )
}


