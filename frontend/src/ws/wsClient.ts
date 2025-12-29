import { wsBaseUrl } from '../lib/env'

export type Handler = (payload: unknown) => void

export class WsClient {
  private ws?: WebSocket
  private handlers = new Map<string, Set<Handler>>()

  /** Clears registered message handlers only (does not close the WebSocket). Useful for swapping listeners while keeping the connection alive. */
  clearHandlers() {
    this.handlers.clear()
  }

  private emit(type: string, payload: unknown) {
    const set = this.handlers.get(type)
    if (!set) return
    for (const h of set) h(payload)
  }

  connect(room?: string) {
    this.closeSocket()
    const base = wsBaseUrl()
    const url = new URL('/ws', base)
    if (room) url.searchParams.set('room', room)

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

  private closeSocket() {
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
  }

  disconnect() {
    this.clearHandlers()
    this.closeSocket()
  }
}


