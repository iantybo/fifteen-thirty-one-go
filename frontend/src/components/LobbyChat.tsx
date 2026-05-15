import { forwardRef, useEffect, useImperativeHandle, useMemo, useRef, useState } from 'react'
import type { CSSProperties } from 'react'
import { api } from '../api/client'
import type { LobbyChatMessage } from '../api/types'

interface LobbyChatProps {
  lobbyId: number
}

export type LobbyChatHandle = {
  addMessage: (message: LobbyChatMessage) => void
}

// Static styles hoisted to module scope so referential identity is stable
// across renders. Anything that depends on state stays in JSX.
const containerStyle: CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  height: '100%',
  border: '1px solid #ddd',
  borderRadius: '8px',
  backgroundColor: '#fff',
}

const headerStyle: CSSProperties = {
  padding: '12px 16px',
  borderBottom: '1px solid #ddd',
  fontWeight: 'bold',
  fontSize: '1.1em',
}

const messageListStyle: CSSProperties = {
  flex: 1,
  overflowY: 'auto',
  padding: '16px',
  display: 'flex',
  flexDirection: 'column',
  gap: '12px',
}

const emptyStateStyle: CSSProperties = { color: '#999', textAlign: 'center', marginTop: '20px' }
const chatHeaderRowStyle: CSSProperties = { display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }
const usernameStyle: CSSProperties = { color: '#333' }
const timestampStyle: CSSProperties = { color: '#999', fontSize: '0.85em' }

const inputRowContainerStyle: CSSProperties = { padding: '12px 16px', borderTop: '1px solid #ddd' }
const errorStyle: CSSProperties = { color: 'crimson', marginBottom: '8px', fontSize: '0.9em' }
const inputRowStyle: CSSProperties = { display: 'flex', gap: '8px' }
const inputStyle: CSSProperties = {
  flex: 1,
  padding: '8px 12px',
  border: '1px solid #ddd',
  borderRadius: '4px',
  fontSize: '1em',
}

const systemMsgStyle: CSSProperties = { fontStyle: 'italic', color: '#666', fontSize: '0.9em' }
const joinMsgStyle: CSSProperties = { fontStyle: 'italic', color: '#4a9eff', fontSize: '0.9em' }
const leaveMsgStyle: CSSProperties = { fontStyle: 'italic', color: '#999', fontSize: '0.9em' }
const emptyTypeStyle: CSSProperties = {}

function styleForMessageType(messageType: string): CSSProperties {
  switch (messageType) {
    case 'system':
      return systemMsgStyle
    case 'join':
      return joinMsgStyle
    case 'leave':
      return leaveMsgStyle
    default:
      return emptyTypeStyle
  }
}

function formatTime(timestamp: string): string {
  return new Date(timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
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

  useImperativeHandle(
    ref,
    () => ({
      addMessage: (message: LobbyChatMessage) => setMessages((prev) => [...prev, message]),
    }),
    [],
  )

  const handleSend = async () => {
    if (!inputMessage.trim() || isSending) return

    setError(null)
    setIsSending(true)
    try {
      await api.sendLobbyChatMessage(lobbyId, { message: inputMessage.trim() })
      // Message will be broadcast via WebSocket and received by all clients
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

  const canSend = inputMessage.trim().length > 0 && !isSending
  const sendButtonStyle = useMemo<CSSProperties>(
    () => ({
      padding: '8px 16px',
      backgroundColor: canSend ? '#4a9eff' : '#ddd',
      color: canSend ? '#fff' : '#999',
      border: 'none',
      borderRadius: '4px',
      cursor: canSend ? 'pointer' : 'not-allowed',
      fontWeight: 'bold',
    }),
    [canSend],
  )

  return (
    <div style={containerStyle}>
      <div style={headerStyle}>Lobby Chat</div>

      <div ref={chatContainerRef} style={messageListStyle}>
        {messages.length === 0 && <div style={emptyStateStyle}>No messages yet. Start the conversation!</div>}
        {messages.map((msg) => {
          const isChat = msg.message_type === 'chat'
          const rowStyle: CSSProperties = {
            ...styleForMessageType(msg.message_type),
            padding: isChat ? '8px 12px' : '4px 8px',
            backgroundColor: isChat ? '#f5f5f5' : 'transparent',
            borderRadius: '6px',
          }
          return (
            <div key={msg.id} style={rowStyle}>
              {isChat && (
                <div style={chatHeaderRowStyle}>
                  <strong style={usernameStyle}>{msg.username}</strong>
                  <span style={timestampStyle}>{formatTime(msg.created_at)}</span>
                </div>
              )}
              <div>{msg.message}</div>
            </div>
          )
        })}
        <div ref={messagesEndRef} />
      </div>

      <div style={inputRowContainerStyle}>
        {error && <div style={errorStyle}>{error}</div>}
        <div style={inputRowStyle}>
          <input
            type="text"
            value={inputMessage}
            onChange={(e) => setInputMessage(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message..."
            disabled={isSending}
            style={inputStyle}
          />
          <button onClick={handleSend} disabled={!canSend} style={sendButtonStyle}>
            {isSending ? 'Sending...' : 'Send'}
          </button>
        </div>
      </div>
    </div>
  )
})

// Note: If you need websocket-driven lobby chat, prefer rendering <LobbyChat ref={...} />
// and calling ref.current.addMessage(...) from the websocket handler.
