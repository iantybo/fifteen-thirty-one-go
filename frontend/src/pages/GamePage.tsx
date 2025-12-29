import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../api/client'
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
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    if (!user || !isValidId) return
    let cancelled = false

    // Fetch an initial snapshot immediately so the UI isn't "null" until a WS update arrives.
    void (async () => {
      try {
        const snap = await api.getGame(gameId)
        if (!cancelled) setLastMsg(snap)
      } catch (e: unknown) {
        if (!cancelled) setErr(e instanceof Error ? e.message : 'failed to load game')
      }
    })()

    ws.connect(`game:${gameId}`)
    const offOpen = ws.on('ws_open', () => setStatus('connected'))
    const offClose = ws.on('ws_close', () => setStatus('disconnected'))
    const offUpdate = ws.on('game_update', (p) => setLastMsg(p))
    return () => {
      cancelled = true
      offOpen()
      offClose()
      offUpdate()
      ws.disconnect()
    }
  }, [user, gameId, isValidId, ws])

  return (
    <div style={{ maxWidth: 900, margin: '24px auto', padding: '0 16px' }}>
      <h1>Game {isValidId ? gameId : 'Invalid ID'}</h1>
      <div>Status: {status}</div>
      {err && <div style={{ color: 'crimson', marginTop: 8 }}>{err}</div>}
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


