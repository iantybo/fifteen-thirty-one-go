import { useEffect, useState, useRef, useCallback } from 'react'
import { WsClient } from '../ws/wsClient'
import { api } from '../api/client'
import type { PresenceStatus } from '../api/types'

export function usePresence() {
  const [onlineUsers, setOnlineUsers] = useState<PresenceStatus[]>([])
  const [connected, setConnected] = useState(false)
  const wsRef = useRef<WsClient | null>(null)
  const heartbeatRef = useRef<number | null>(null)

  const updatePresence = useCallback(async (status: 'online' | 'away' | 'in_game' | 'offline') => {
    try {
      await api.updatePresence({ status })
    } catch (error) {
      console.error('Failed to update presence:', error)
    }
  }, [])

  useEffect(() => {
    const ws = new WsClient()
    wsRef.current = ws

    ws.on('ws_open', () => {
      setConnected(true)
      updatePresence('online')
    })

    ws.on('ws_close', () => {
      setConnected(false)
    })

    ws.on('ws_error', (error) => {
      console.error('WebSocket error:', error)
      setConnected(false)
    })

    ws.on('player:presence_changed', (payload) => {
      const presence = payload as PresenceStatus
      setOnlineUsers((prev) => {
        if (presence.status === 'offline') {
          return prev.filter((p) => p.user_id !== presence.user_id)
        }
        const existing = prev.findIndex((p) => p.user_id === presence.user_id)
        if (existing !== -1) {
          const updated = [...prev]
          updated[existing] = presence
          return updated
        } else {
          return [...prev, presence]
        }
      })
    })

    ws.connect('lobby:global')

    // Send heartbeat every 30 seconds
    heartbeatRef.current = window.setInterval(() => {
      api.presenceHeartbeat().catch((err) => {
        console.error('Heartbeat failed:', err)
      })
    }, 30000)

    return () => {
      if (heartbeatRef.current) {
        clearInterval(heartbeatRef.current)
      }
      ws.disconnect()
      updatePresence('offline').catch((err) => {
        console.error('Failed to set offline status:', err)
      })
    }
    // updatePresence is stable (useCallback with empty deps).
  }, [])

  return {
    onlineUsers,
    connected,
    updatePresence
  }
}
