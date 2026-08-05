/**
 * A controllable fake `WsClient` for engine/integration tests.
 *
 * The engine uses only a narrow slice of the real `WsClient` surface —
 * `connect`, `on`, `send`, `disconnect` — plus, for tests, a way to *drive*
 * events into the engine. This double mirrors the class that previously lived
 * inline inside `engine.test.ts`, extracted here so the integration suites can
 * share one implementation instead of re-declaring it in every file.
 *
 * Usage:
 *  - Inject via `makeWs: () => ws as unknown as WsClient`.
 *  - After the engine wires its handlers in `start()`, call `ws.fire('ws_open')`,
 *    `ws.fire('ws_close')` or `ws.fire('game_update')` to simulate the server.
 *  - `handlers` is exposed so a test can assert which events the engine subscribed
 *    to, and `connected` reflects connect/disconnect for basic lifecycle checks.
 *
 * Handlers are typed with the same `Handler` alias the real client exports, so a
 * mismatch between the fake and the production event signature is a compile
 * error rather than a silent divergence.
 */

import type { Handler } from '../../ws/wsClient'

export class FakeWs {
  /** Registered handlers keyed by event type (e.g. 'ws_open', 'game_update'). */
  readonly handlers = new Map<string, Set<Handler>>()
  /** Whether `connect()` has been called and `disconnect()` has not. */
  connected = false

  connect(): void {
    this.connected = true
  }

  on(type: string, h: Handler): () => void {
    const set = this.handlers.get(type) ?? new Set<Handler>()
    set.add(h)
    this.handlers.set(type, set)
    return () => set.delete(h)
  }

  send(): void {}

  disconnect(): void {
    this.connected = false
    this.handlers.clear()
  }

  /** Fire an event to every handler registered for `type`. */
  fire(type: string, payload?: unknown): void {
    for (const h of this.handlers.get(type) ?? []) h(payload)
  }
}
