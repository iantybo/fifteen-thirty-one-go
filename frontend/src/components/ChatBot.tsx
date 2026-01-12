import { useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import type { ChatbotMessage, CribbageStage } from '../api/types'

interface ChatBotProps {
  gameId: number
  stage: CribbageStage
  scores: number[]
  handSize: number
}

export function ChatBot({ gameId, stage, scores, handSize }: ChatBotProps) {
  const [messages, setMessages] = useState<ChatbotMessage[]>([])
  const [inputMessage, setInputMessage] = useState('')
  const [isSending, setIsSending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [isMinimized, setIsMinimized] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    if (!isMinimized) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
    }
  }, [messages, isMinimized])

  const handleSend = async () => {
    if (!inputMessage.trim() || isSending) return

    const userMessage: ChatbotMessage = {
      id: `user-${Date.now()}`,
      role: 'user',
      content: inputMessage.trim(),
      timestamp: new Date().toISOString(),
    }

    setMessages((prev) => [...prev, userMessage])
    setInputMessage('')
    setError(null)
    setIsSending(true)

    try {
      const res = await api.sendChatbotMessage(gameId, {
        message: userMessage.content,
        game_context: {
          game_id: gameId,
          stage,
          scores,
          hand_size: handSize,
        },
      })

      const assistantMessage: ChatbotMessage = {
        id: `assistant-${Date.now()}`,
        role: 'assistant',
        content: res.message,
        timestamp: res.timestamp,
      }

      setMessages((prev) => [...prev, assistantMessage])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to get response')
      // Remove the user message on error
      setMessages((prev) => prev.filter((m) => m.id !== userMessage.id))
      setInputMessage(userMessage.content)
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

  if (isMinimized) {
    return (
      <div
        style={{
          position: 'fixed',
          bottom: 20,
          right: 20,
          zIndex: 1000,
        }}
      >
        <button
          onClick={() => setIsMinimized(false)}
          style={{
            padding: '12px 20px',
            backgroundColor: '#4a9eff',
            color: 'white',
            border: 'none',
            borderRadius: '24px',
            fontSize: '16px',
            fontWeight: 'bold',
            cursor: 'pointer',
            boxShadow: '0 4px 12px rgba(74, 158, 255, 0.4)',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
          }}
        >
          <span>💬</span>
          <span>AI Helper</span>
        </button>
      </div>
    )
  }

  return (
    <div
      style={{
        position: 'fixed',
        bottom: 20,
        right: 20,
        width: 380,
        height: 500,
        display: 'flex',
        flexDirection: 'column',
        border: '1px solid #ddd',
        borderRadius: '12px',
        backgroundColor: '#fff',
        boxShadow: '0 8px 24px rgba(0, 0, 0, 0.15)',
        zIndex: 1000,
      }}
    >
      <div
        style={{
          padding: '12px 16px',
          borderBottom: '1px solid #ddd',
          fontWeight: 'bold',
          fontSize: '1.1em',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          backgroundColor: '#f8f9fa',
          borderTopLeftRadius: '12px',
          borderTopRightRadius: '12px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span>💬</span>
          <span>AI Helper</span>
        </div>
        <button
          onClick={() => setIsMinimized(true)}
          style={{
            background: 'none',
            border: 'none',
            fontSize: '20px',
            cursor: 'pointer',
            color: '#666',
            padding: '0 4px',
          }}
          title="Minimize"
        >
          −
        </button>
      </div>

      <div
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
            <div style={{ fontSize: '2em', marginBottom: '8px' }}>👋</div>
            <div>Hi! I'm your AI helper.</div>
            <div style={{ marginTop: '8px', fontSize: '0.9em' }}>
              Ask me about game rules, strategies, or tips!
            </div>
          </div>
        )}
        {messages.map((msg) => (
          <div
            key={msg.id}
            style={{
              padding: '10px 14px',
              backgroundColor: msg.role === 'user' ? '#4a9eff' : '#f0f0f0',
              color: msg.role === 'user' ? 'white' : '#333',
              borderRadius: '12px',
              maxWidth: '85%',
              alignSelf: msg.role === 'user' ? 'flex-end' : 'flex-start',
              wordWrap: 'break-word',
            }}
          >
            <div style={{ whiteSpace: 'pre-wrap' }}>{msg.content}</div>
            <div
              style={{
                fontSize: '0.75em',
                marginTop: '4px',
                opacity: 0.7,
                textAlign: msg.role === 'user' ? 'right' : 'left',
              }}
            >
              {formatTime(msg.timestamp)}
            </div>
          </div>
        ))}
        {isSending && (
          <div
            style={{
              padding: '10px 14px',
              backgroundColor: '#f0f0f0',
              borderRadius: '12px',
              maxWidth: '85%',
              alignSelf: 'flex-start',
              display: 'flex',
              gap: '4px',
            }}
          >
            <div className="typing-indicator">
              <span>●</span>
              <span>●</span>
              <span>●</span>
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      <div
        style={{
          padding: '12px 16px',
          borderTop: '1px solid #ddd',
          backgroundColor: '#f8f9fa',
          borderBottomLeftRadius: '12px',
          borderBottomRightRadius: '12px',
        }}
      >
        {error && (
          <div
            style={{
              color: '#d32f2f',
              fontSize: '0.9em',
              marginBottom: '8px',
              padding: '6px 8px',
              backgroundColor: '#ffebee',
              borderRadius: '4px',
            }}
          >
            {error}
          </div>
        )}
        <div style={{ display: 'flex', gap: '8px' }}>
          <input
            type="text"
            value={inputMessage}
            onChange={(e) => setInputMessage(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Ask a question..."
            disabled={isSending}
            style={{
              flex: 1,
              padding: '8px 12px',
              border: '1px solid #ddd',
              borderRadius: '20px',
              fontSize: '14px',
              outline: 'none',
            }}
          />
          <button
            onClick={handleSend}
            disabled={!inputMessage.trim() || isSending}
            style={{
              padding: '8px 16px',
              backgroundColor: inputMessage.trim() && !isSending ? '#4a9eff' : '#ccc',
              color: 'white',
              border: 'none',
              borderRadius: '20px',
              fontSize: '14px',
              fontWeight: 'bold',
              cursor: inputMessage.trim() && !isSending ? 'pointer' : 'not-allowed',
            }}
          >
            Send
          </button>
        </div>
      </div>

      <style>
        {`
          @keyframes typing {
            0%, 60%, 100% { opacity: 0.3; }
            30% { opacity: 1; }
          }
          .typing-indicator span {
            animation: typing 1.4s infinite;
            display: inline-block;
            margin: 0 2px;
          }
          .typing-indicator span:nth-child(2) {
            animation-delay: 0.2s;
          }
          .typing-indicator span:nth-child(3) {
            animation-delay: 0.4s;
          }
        `}
      </style>
    </div>
  )
}
