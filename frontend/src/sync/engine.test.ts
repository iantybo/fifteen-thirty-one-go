import { describe, it, expect, beforeEach, vi } from 'vitest'
import { SyncEngine, toWireMove, isPermanentError, type EngineDeps } from './engine'
import { MemoryStore } from './queue'
import { cardToCode } from './reducer'
import { ApiError } from '../lib/http'
import type { GameSnapshot } from '../api/types'
import { peggingState, discardState, snapshotFor } from './__fixtures__/state'
import type { Handler } from '../ws/wsClient'

/**
 * A controllable fake WsClient. We only need `connect`/`on`/`disconnect` for the
 * engine, plus a way for the test to fire events.
 */
class FakeWs {
  handlers = new Map<string, Set<Handler>>()
  connected = false
  connect() {
    this.connected = true
  }
  on(type: string, h: Handler) {
    const set = this.handlers.get(type) ?? new Set<Handler>()
    set.add(h)
    this.handlers.set(type, set)
    return () => set.delete(h)
  }
  send() {}
  disconnect() {
    this.connected = false
    this.handlers.clear()
  }
  fire(type: string, payload?: unknown) {
    for (const h of this.handlers.get(type) ?? []) h(payload)
  }
}

const MY_USER = 9
const GAME = 1

type GameApi = NonNullable<EngineDeps['gameApi']>

/** Build engine deps wired to a fake WS + stub API + synchronous timers. */
function makeDeps(api: Partial<GameApi>, ws: FakeWs) {
  const timers: Array<{ fn: () => void; ms: number }> = []
  const deps: Partial<EngineDeps> = {
    now: () => 1000,
    // Run zero-delay timers synchronously via a microtask; queue delayed ones so
    // tests can flush them explicitly.
    setTimer: (fn, ms) => {
      if (ms === 0) {
        queueMicrotask(fn)
        return () => {}
      }
      const entry = { fn, ms }
      timers.push(entry)
      return () => {
        const i = timers.indexOf(entry)
        if (i >= 0) timers.splice(i, 1)
      }
    },
    store: new MemoryStore(),
    makeWs: () => ws as unknown as import('../ws/wsClient').WsClient,
    gameApi: {
      getGame: api.getGame ?? (async () => snapshotFor(peggingState(), MY_USER)),
      moveGame: api.moveGame ?? (async () => ({})),
      nextHand: api.nextHand ?? (async () => undefined),
    },
  }
  return { deps, timers }
}

/** Await all pending microtasks so optimistic/flush effects settle. */
async function tick() {
  await Promise.resolve()
  await Promise.resolve()
  await Promise.resolve()
}

describe('toWireMove', () => {
  it('maps engine actions to wire moves', () => {
    expect(toWireMove({ kind: 'play_card', card: '5H' })).toEqual({ type: 'play_card', card: '5H' })
    expect(toWireMove({ kind: 'discard', cards: ['AH'] })).toEqual({ type: 'discard', cards: ['AH'] })
    expect(toWireMove({ kind: 'go' })).toEqual({ type: 'go' })
    expect(toWireMove({ kind: 'ready_next_hand' })).toBeNull()
  })
})

describe('isPermanentError', () => {
  it('treats 4xx (except 408/429) as permanent', () => {
    expect(isPermanentError(new ApiError('bad', 400))).toBe(true)
    expect(isPermanentError(new ApiError('conflict', 409))).toBe(true)
    expect(isPermanentError(new ApiError('rate', 429))).toBe(false)
    expect(isPermanentError(new ApiError('timeout', 408))).toBe(false)
    expect(isPermanentError(new ApiError('boom', 500))).toBe(false)
    expect(isPermanentError(new Error('network'))).toBe(false)
  })
})

describe('SyncEngine — optimistic dispatch', () => {
  let ws: FakeWs

  beforeEach(() => {
    ws = new FakeWs()
  })

  it('applies a play optimistically before the server confirms', async () => {
    const getGame = vi.fn(async () => snapshotFor(peggingState(), MY_USER))
    const moveGame = vi.fn(async () => ({}))
    const { deps } = makeDeps({ getGame, moveGame }, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)

    await engine.start()
    await tick()

    // Baseline: 5H,6H in hand, total 10.
    expect(engine.getState().optimisticState?.pegging_total).toBe(10)

    engine.dispatch({ kind: 'play_card', card: '5H' })
    // Optimistic update is synchronous — no await needed for the prediction.
    const st = engine.getState().optimisticState!
    expect(st.pegging_total).toBe(15)
    expect(st.hands[0].map(cardToCode)).toEqual(['6H'])
    // The action was queued.
    expect(engine.getState().queue.some((a) => a.action.kind === 'play_card')).toBe(true)

    engine.stop()
  })

  it('flushes queued actions to the server', async () => {
    const moveGame = vi.fn(async () => ({}))
    const { deps } = makeDeps({ moveGame }, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    engine.dispatch({ kind: 'play_card', card: '5H' })
    await tick()

    expect(moveGame).toHaveBeenCalledWith(GAME, { type: 'play_card', card: '5H' })
    engine.stop()
  })

  it('rolls back an action the server permanently rejects', async () => {
    const moveGame = vi.fn(async () => {
      throw new ApiError('not your turn', 409)
    })
    const { deps } = makeDeps({ moveGame }, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    const id = engine.dispatch({ kind: 'play_card', card: '5H' })
    // Optimistically applied.
    expect(engine.getState().optimisticState?.pegging_total).toBe(15)
    await tick()

    // After rejection: action dropped from the queue and prediction rolled back.
    const state = engine.getState()
    expect(state.queue.find((a) => a.clientId === id)).toBeUndefined()
    expect(state.optimisticState?.pegging_total).toBe(10)
    engine.stop()
  })

  it('reconciles to the authoritative snapshot on game_update', async () => {
    // First snapshot: total 10. After the update: server advanced to total 15
    // with the card gone from the hand.
    let call = 0
    const getGame = vi.fn(async (): Promise<GameSnapshot> => {
      call += 1
      if (call === 1) return snapshotFor(peggingState({ pegging_total: 10 }), MY_USER)
      return snapshotFor(peggingState({ pegging_total: 15, hands: [[], []], current_index: 1 }), MY_USER)
    })
    const { deps } = makeDeps({ getGame }, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()
    expect(engine.getState().optimisticState?.pegging_total).toBe(10)

    ws.fire('game_update')
    await tick()

    expect(engine.getState().optimisticState?.pegging_total).toBe(15)
    expect(engine.getState().revision).toBeGreaterThan(0)
    engine.stop()
  })

  it('ignores a stale snapshot that would regress the board', async () => {
    const engine = new SyncEngine(GAME, MY_USER, makeDeps({}, ws).deps)
    await engine.start()
    await tick()

    const before = engine.getState().revision
    // Manually reconcile with an older revision — must be ignored.
    const res = engine.reconcile(snapshotFor(peggingState()), before - 1, [], [])
    expect(res.ignoredStale).toBe(true)
    expect(engine.getState().revision).toBe(before)
    engine.stop()
  })

  it('tracks connection state from WS events', async () => {
    const engine = new SyncEngine(GAME, MY_USER, makeDeps({}, ws).deps)
    await engine.start()
    await tick()

    ws.fire('ws_open')
    expect(engine.getState().connected).toBe(true)
    ws.fire('ws_close')
    expect(engine.getState().connected).toBe(false)
    engine.stop()
  })

  it('dispatches discard actions optimistically', async () => {
    const getGame = vi.fn(async () => snapshotFor(discardState(), MY_USER))
    const { deps } = makeDeps({ getGame }, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    engine.dispatch({ kind: 'discard', cards: ['AH', '2H'] })
    const st = engine.getState().optimisticState!
    expect(st.hands[0].map(cardToCode)).toEqual(['3H', '4H', '5H', '6H'])
    expect(st.discard_completed[0]).toBe(true)
    engine.stop()
  })
})
