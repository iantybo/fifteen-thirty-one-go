import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/auth'

export function LobbyDetailPage() {
  const { id } = useParams()
  const lobbyId = Number(id)
  const isValidId = Number.isFinite(lobbyId) && lobbyId > 0
  const { token } = useAuth()
  const nav = useNavigate()
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function join() {
    if (!token || !isValidId) return
    setErr(null)
    setBusy(true)
    try {
      const res = await api.joinLobby(token, lobbyId)
      nav(`/games/${res.game_id}`, { replace: true })
    } catch (e: any) {
      setErr(e?.message ?? 'failed to join')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{ maxWidth: 700, margin: '24px auto', padding: '0 16px' }}>
      <h1>Lobby {isValidId ? lobbyId : 'Invalid ID'}</h1>
      <button disabled={busy || !isValidId} onClick={join}>
        {busy ? 'Joining…' : 'Join lobby'}
      </button>
      {err && <div style={{ color: 'crimson', marginTop: 8 }}>{err}</div>}
    </div>
  )
}


