import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useAuth } from '../auth/auth'
import { WsClient } from '../ws/wsClient'

export function GamePage() {
  const { id } = useParams()
  const gameId = Number(id)
  const isValidId = Number.isFinite(gameId) && gameId > 0
  const { user } = useAuth()
  const ws = useMemo(() => new WsClient(), [])
  const [status, setStatus] = useState<string>('disconnected')
  const [lastMsg, setLastMsg] = useState<unknown>(null)

  useEffect(() => {
    if (!user || !isValidId) return
    ws.connect(`game:${gameId}`)
    const offOpen = ws.on('ws_open', () => setStatus('connected'))
    const offClose = ws.on('ws_close', () => setStatus('disconnected'))
    const offUpdate = ws.on('game_update', (p) => setLastMsg(p))
    return () => {
      offOpen()
      offClose()
      offUpdate()
      ws.disconnect()
      setStatus('disconnected')
    }
  }, [user, gameId, isValidId, ws])

  return (
    <div style={{ maxWidth: 900, margin: '24px auto', padding: '0 16px' }}>
      <h1>Game {isValidId ? gameId : 'Invalid ID'}</h1>
      <div>Status: {status}</div>
      <h2 style={{ marginTop: 16 }}>Latest snapshot</h2>
      <pre style={{ background: '#111', color: '#ddd', padding: 12, borderRadius: 6, overflow: 'auto' }}>
        {JSON.stringify(lastMsg, null, 2)}
      </pre>
      <p style={{ opacity: 0.8 }}>
        Next: implement proper game board UI + move sending (Phase 7.14).
      </p>
    </div>
  )
}


