import { wsBaseUrl } from '../lib/env'

export type Handler = (payload: unknown) => void

export class WsClient {
  private ws?: WebSocket
  private handlers = new Map<string, Set<Handler>>()

  private emit(type: string, payload: unknown) {
    const set = this.handlers.get(type)
    if (!set) return
    for (const h of set) h(payload)
  }

  connect(token: string, room?: string) {
    this.disconnect()
    const base = wsBaseUrl()
    const url = new URL('/ws', base)
    if (room) url.searchParams.set('room', room)

    // Prefer Authorization header, but browsers don't allow it for WebSocket.
    // Backend supports query tokens only when WS_ALLOW_QUERY_TOKENS=true.
    // WARNING: Query-string tokens can leak via access logs, proxy logs, and browser history.
    // For production, prefer an ephemeral one-time WS token exchange or another auth mechanism.
    url.searchParams.set('token', token)

    const ws = new WebSocket(url.toString())
    this.ws = ws

    ws.onopen = () => {
      this.emit('ws_open', undefined)
    }
    ws.onerror = (evt) => {
      this.emit('ws_error', evt)
    }
    ws.onclose = (evt) => {
      // If this is the active socket, clear it.
      if (this.ws === ws) this.ws = undefined
      this.emit('ws_close', evt)
    }
    ws.onmessage = (evt) => {
      let msg: unknown
      try {
        msg = JSON.parse(evt.data)
      } catch {
        return
      }
      if (!msg || typeof msg !== 'object') return
      const m = msg as Record<string, unknown>
      const type = typeof m.type === 'string' ? m.type : ''
      if (!type) return
      this.emit(type, m.payload)
    }
  }

  on(type: string, handler: Handler) {
    const set = this.handlers.get(type) ?? new Set<Handler>()
    set.add(handler)
    this.handlers.set(type, set)
    return () => {
      const s = this.handlers.get(type)
      if (!s) return
      s.delete(handler)
    }
  }

  send(type: string, payload: unknown) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return
    this.ws.send(JSON.stringify({ type, payload }))
  }

  disconnect() {
    if (!this.ws) return
    const ws = this.ws
    // Detach event handlers to avoid retaining closures.
    ws.onopen = null
    ws.onclose = null
    ws.onerror = null
    ws.onmessage = null
    try {
      ws.close()
    } catch {
      // ignore
    }
    this.ws = undefined
    this.handlers.clear()
  }
}


