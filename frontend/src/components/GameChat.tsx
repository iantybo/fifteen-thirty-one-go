import { useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import type { GameChatMessage } from '../api/types'
import type { WsClient } from '../ws/wsClient'

interface GameChatProps {
  gameId: number
  ws: WsClient
  wsConnected: boolean
}

export function GameChat({ gameId, ws, wsConnected }: GameChatProps) {
  const [messages, setMessages] = useState<GameChatMessage[]>([])
  const [inputMessage, setInputMessage] = useState('')
  const [isSending, setIsSending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    let cancelled = false
    async function loadHistory() {
      try {
        const res = await api.getGameChatHistory(gameId)
        if (!cancelled) setMessages(res.messages)
      } catch (err) {
        if (!cancelled) console.error('Failed to load game chat history:', err)
      }
    }
    void loadHistory()
    return () => {
      cancelled = true
    }
  }, [gameId])

  useEffect(() => {
    return ws.on('game:chat', (payload) => {
      const msg = payload as GameChatMessage
      if (!msg || typeof msg !== 'object') return
      if (msg.game_id !== gameId) return
      setMessages((prev) => [...prev, msg])
    })
  }, [ws, gameId])

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp)
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }

  const handleSend = async () => {
    const trimmed = inputMessage.trim()
    if (!trimmed || isSending) return

    setError(null)
    setIsSending(true)
    try {
      // Prefer WS for low-latency, but always fall back to HTTP.
      if (wsConnected) {
        ws.send('game:send_message', { game_id: gameId, message: trimmed })
      } else {
        await api.sendGameChatMessage(gameId, { message: trimmed })
      }
      setInputMessage('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send message')
    } finally {
      setIsSending(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: 260,
        border: '1px solid #e2e8f0',
        borderRadius: 12,
        background: '#ffffff',
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          padding: '10px 12px',
          borderBottom: '1px solid #e2e8f0',
          fontWeight: 900,
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'baseline',
          gap: 10,
        }}
      >
        <div>Game Chat</div>
        <div style={{ fontSize: 12, opacity: 0.7 }}>{wsConnected ? 'Live' : 'Offline'}</div>
      </div>

      <div
        style={{
          flex: 1,
          overflowY: 'auto',
          padding: 12,
          display: 'flex',
          flexDirection: 'column',
          gap: 10,
          background: '#f8fafc',
        }}
      >
        {messages.length === 0 ? <div style={{ opacity: 0.7, textAlign: 'center' }}>No messages yet.</div> : null}
        {messages.map((m) => (
          <div
            key={m.id}
            style={{
              padding: '8px 10px',
              borderRadius: 10,
              border: '1px solid rgba(15,23,42,0.10)',
              background: '#ffffff',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10, marginBottom: 2 }}>
              <div style={{ fontWeight: 800 }}>{m.username}</div>
              <div style={{ fontSize: 12, opacity: 0.65 }}>{formatTime(m.created_at)}</div>
            </div>
            <div style={{ overflowWrap: 'anywhere' }}>{m.message}</div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>

      <div style={{ padding: 10, borderTop: '1px solid #e2e8f0' }}>
        {error ? <div style={{ color: 'crimson', marginBottom: 8, fontSize: 12 }}>{error}</div> : null}
        <div style={{ display: 'flex', gap: 8 }}>
          <input
            type="text"
            value={inputMessage}
            onChange={(e) => setInputMessage(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message…"
            disabled={isSending}
            style={{
              flex: 1,
              padding: '8px 10px',
              border: '1px solid #cbd5e1',
              borderRadius: 8,
              fontSize: 14,
            }}
          />
          <button
            type="button"
            onClick={handleSend}
            disabled={!inputMessage.trim() || isSending}
            style={{
              padding: '8px 12px',
              borderRadius: 8,
              border: 'none',
              background: inputMessage.trim() && !isSending ? '#2563eb' : '#cbd5e1',
              color: inputMessage.trim() && !isSending ? '#ffffff' : '#475569',
              fontWeight: 900,
              cursor: inputMessage.trim() && !isSending ? 'pointer' : 'not-allowed',
            }}
          >
            Send
          </button>
        </div>
      </div>
    </div>
  )
}


