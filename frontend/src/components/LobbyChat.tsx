import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react'
import { api } from '../api/client'
import type { LobbyChatMessage } from '../api/types'

interface LobbyChatProps {
  lobbyId: number
}

export type LobbyChatHandle = {
  addMessage: (message: LobbyChatMessage) => void
}

export const LobbyChat = forwardRef<LobbyChatHandle, LobbyChatProps>(function LobbyChat({ lobbyId }: LobbyChatProps, ref) {
  const [messages, setMessages] = useState<LobbyChatMessage[]>([])
  const [inputMessage, setInputMessage] = useState('')
  const [isSending, setIsSending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const chatContainerRef = useRef<HTMLDivElement>(null)

  // Load chat history on mount
  useEffect(() => {
    let cancelled = false
    async function loadHistory() {
      try {
        const res = await api.getLobbyChatHistory(lobbyId)
        if (!cancelled) setMessages(res.messages)
      } catch (err) {
        if (!cancelled) console.error('Failed to load chat history:', err)
      }
    }
    void loadHistory()
    return () => {
      cancelled = true
    }
  }, [lobbyId])

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const addMessage = (message: LobbyChatMessage) => {
    setMessages((prev) => [...prev, message])
  }

  useImperativeHandle(ref, () => ({ addMessage }), [])

  const handleSend = async () => {
    if (!inputMessage.trim() || isSending) return

    setError(null)
    setIsSending(true)
    try {
      await api.sendLobbyChatMessage(lobbyId, { message: inputMessage.trim() })
      // Message will be broadcast via WebSocket and received by all clients
      // We could optimistically add it here, but the broadcast will handle it
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

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp)
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }

  const getMessageStyle = (messageType: string) => {
    switch (messageType) {
      case 'system':
        return { fontStyle: 'italic', color: '#666', fontSize: '0.9em' }
      case 'join':
        return { fontStyle: 'italic', color: '#4a9eff', fontSize: '0.9em' }
      case 'leave':
        return { fontStyle: 'italic', color: '#999', fontSize: '0.9em' }
      default:
        return {}
    }
  }

  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      height: '100%',
      border: '1px solid #ddd',
      borderRadius: '8px',
      backgroundColor: '#fff',
    }}>
      <div style={{
        padding: '12px 16px',
        borderBottom: '1px solid #ddd',
        fontWeight: 'bold',
        fontSize: '1.1em',
      }}>
        Lobby Chat
      </div>

      <div
        ref={chatContainerRef}
        style={{
          flex: 1,
          overflowY: 'auto',
          padding: '16px',
          display: 'flex',
          flexDirection: 'column',
          gap: '12px',
        }}
      >
        {messages.length === 0 && (
          <div style={{ color: '#999', textAlign: 'center', marginTop: '20px' }}>
            No messages yet. Start the conversation!
          </div>
        )}
        {messages.map((msg) => (
          <div
            key={msg.id}
            style={{
              ...getMessageStyle(msg.message_type),
              padding: msg.message_type === 'chat' ? '8px 12px' : '4px 8px',
              backgroundColor: msg.message_type === 'chat' ? '#f5f5f5' : 'transparent',
              borderRadius: '6px',
            }}
          >
            {msg.message_type === 'chat' && (
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
                <strong style={{ color: '#333' }}>{msg.username}</strong>
                <span style={{ color: '#999', fontSize: '0.85em' }}>{formatTime(msg.created_at)}</span>
              </div>
            )}
            <div>{msg.message}</div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>

      <div style={{
        padding: '12px 16px',
        borderTop: '1px solid #ddd',
      }}>
        {error && (
          <div style={{ color: 'crimson', marginBottom: '8px', fontSize: '0.9em' }}>
            {error}
          </div>
        )}
        <div style={{ display: 'flex', gap: '8px' }}>
          <input
            type="text"
            value={inputMessage}
            onChange={(e) => setInputMessage(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message..."
            disabled={isSending}
            style={{
              flex: 1,
              padding: '8px 12px',
              border: '1px solid #ddd',
              borderRadius: '4px',
              fontSize: '1em',
            }}
          />
          <button
            onClick={handleSend}
            disabled={!inputMessage.trim() || isSending}
            style={{
              padding: '8px 16px',
              backgroundColor: inputMessage.trim() && !isSending ? '#4a9eff' : '#ddd',
              color: inputMessage.trim() && !isSending ? '#fff' : '#999',
              border: 'none',
              borderRadius: '4px',
              cursor: inputMessage.trim() && !isSending ? 'pointer' : 'not-allowed',
              fontWeight: 'bold',
            }}
          >
            {isSending ? 'Sending...' : 'Send'}
          </button>
        </div>
      </div>
    </div>
  )
})

// Note: If you need websocket-driven lobby chat, prefer rendering <LobbyChat ref={...} />
// and calling ref.current.addMessage(...) from the websocket handler.

