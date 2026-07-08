/**
 * Integration: dispatch → flush.
 *
 * These tests wire a *real* `SyncEngine` to the fake transport + fake WS + the
 * synchronous-timer harness, exercising the full path a UI dispatch travels:
 * optimistic apply → durable enqueue → ordered flush → reconcile. Unlike the
 * unit tests for `queue`/`reducer`/`reconciler` in isolation, nothing here is
 * stubbed at the seam under test — we assert on what the engine actually asks
 * the transport to do, and in what order.
 *
 * Determinism comes entirely from the DI seams (see `EngineDeps`):
 *  - `now` is frozen, so backoff math is reproducible.
 *  - `setTimer` runs zero-delay flushes on a microtask and parks delayed timers
 *    in an array the test can fire by hand — no real clock, no `vi.useFakeTimers`.
 *  - `gameApi` is `FakeTransport`, recording every call in order.
 */

import { describe, it, expect, beforeEach } from 'vitest'
import { SyncEngine, type EngineDeps } from '../engine'
import { MemoryStore } from '../queue'
import { peggingState, snapshotFor } from '../__fixtures__/state'
import { FakeWs } from '../__fixtures__/ws'
import { FakeTransport } from '../__fixtures__/transport'
import type { GameSnapshot } from '../../api/types'

const MY_USER = 9
const GAME = 1

/**
 * Build engine deps wired to a fake WS + fake transport + synchronous timers.
 * Mirrors the harness in `engine.test.ts`: zero-delay timers run on a microtask,
 * delayed timers are parked so tests can fire them explicitly.
 */
function makeDeps(transport: FakeTransport, ws: FakeWs) {
  const timers: Array<{ fn: () => void; ms: number }> = []
  const deps: Partial<EngineDeps> = {
    now: () => 1000,
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
    makeWs: () => ws as unknown as import('../../ws/wsClient').WsClient,
    gameApi: {
      getGame: transport.getGame,
      moveGame: transport.moveGame,
      nextHand: transport.nextHand,
    },
  }
  return { deps, timers }
}

/** Await pending microtasks so optimistic/flush effects settle. */
async function tick() {
  await Promise.resolve()
  await Promise.resolve()
  await Promise.resolve()
}

/** Count of actions still awaiting server confirmation (pending or inflight). */
function pendingCount(engine: SyncEngine): number {
  return engine.getState().queue.filter((a) => a.status === 'pending' || a.status === 'inflight').length
}

describe('integration: dispatch → flush', () => {
  let ws: FakeWs
  let transport: FakeTransport

  beforeEach(() => {
    ws = new FakeWs()
    // A pegging state with total 10 and 5H,6H in hand for player 0.
    transport = new FakeTransport(snapshotFor(peggingState(), MY_USER))
  })

  it('flushes a single dispatched action to the transport', async () => {
    const { deps } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    engine.dispatch({ kind: 'play_card', card: '5H' })
    await tick()

    expect(transport.moveGameCalls).toEqual([{ gameId: GAME, move: { type: 'play_card', card: '5H' } }])
    engine.stop()
  })

  it('flushes multiple sequential dispatches in author order', async () => {
    // Server never advances here, so both plays stay applicable and are sent.
    const { deps } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    engine.dispatch({ kind: 'play_card', card: '5H' })
    engine.dispatch({ kind: 'play_card', card: '6H' })
    await tick()

    // Oldest-first delivery preserves cribbage move ordering.
    expect(transport.moveGameCalls.map((c) => c.move)).toEqual([
      { type: 'play_card', card: '5H' },
      { type: 'play_card', card: '6H' },
    ])
    engine.stop()
  })

  it('empties the queue after the post-flush reconcile confirms the moves', async () => {
    // After both cards are played the server snapshot shows an empty hand and a
    // higher revision; reconcile drops the now-applied actions from the queue.
    let call = 0
    const advancing: FakeTransport = transport
    advancing.getGame = async (gameId: number): Promise<GameSnapshot> => {
      advancing.getGameCalls.push(gameId)
      call += 1
      // First fetch (start) = baseline; later fetches = server has consumed the plays.
      if (call === 1) return snapshotFor(peggingState({ pegging_total: 10 }), MY_USER)
      return snapshotFor(peggingState({ pegging_total: 21, hands: [[], []] }), MY_USER)
    }

    const { deps } = makeDeps(advancing, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    engine.dispatch({ kind: 'play_card', card: '5H' })
    engine.dispatch({ kind: 'play_card', card: '6H' })
    await tick()

    // The reducer can no longer apply plays against an empty hand, so the folded
    // actions are treated as implicitly confirmed and removed during reconcile.
    expect(pendingCount(engine)).toBe(0)
    expect(engine.getState().queue).toHaveLength(0)
    engine.stop()
  })

  it('reflects outstanding actions in the queue before they are delivered', async () => {
    // Park the flush by parking transport responses: hold moveGame until we peek.
    let release!: () => void
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    transport.moveGame = async (gameId: number, move) => {
      transport.moveGameCalls.push({ gameId, move })
      await gate
      return {}
    }

    const { deps } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    engine.dispatch({ kind: 'play_card', card: '5H' })
    // Let the flush start (action becomes inflight) but block inside moveGame.
    await tick()
    expect(pendingCount(engine)).toBe(1)
    expect(engine.getState().queue[0].status).toBe('inflight')

    // Release the transport and let delivery + reconcile complete.
    release()
    await tick()
    expect(pendingCount(engine)).toBe(0)
    engine.stop()
  })

  it('routes a ready_next_hand dispatch to nextHand, not moveGame', async () => {
    const { deps } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    engine.dispatch({ kind: 'ready_next_hand' })
    await tick()

    expect(transport.nextHandCalls).toEqual([GAME])
    expect(transport.moveGameCalls).toHaveLength(0)
    engine.stop()
  })

  it('applies the optimistic prediction before any transport call resolves', async () => {
    // Block moveGame forever: the board must still move on dispatch.
    transport.moveGame = async (gameId: number, move) => {
      transport.moveGameCalls.push({ gameId, move })
      return new Promise<never>(() => {})
    }

    const { deps } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    expect(engine.getState().optimisticState?.pegging_total).toBe(10)
    engine.dispatch({ kind: 'play_card', card: '5H' })
    // Synchronous: no await between dispatch and reading the predicted state.
    expect(engine.getState().optimisticState?.pegging_total).toBe(15)
    engine.stop()
  })
})
