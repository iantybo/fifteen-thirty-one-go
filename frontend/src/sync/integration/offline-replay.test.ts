/**
 * Integration: offline queueing + backoff replay.
 *
 * Simulates a flaky/absent network: `FakeTransport.moveGame` throws a plain
 * `Error` (a *transient* failure — `isPermanentError` returns false for
 * non-4xx). The engine's contract for transient failures (`engine.ts#flush`) is:
 * leave the action `pending`, record an attempt, and schedule a *backoff* flush
 * rather than dropping the move. When the network recovers, the parked flush
 * fires and the queued actions drain in order.
 *
 * The key DI seam here is `setTimer`: zero-delay flushes run on a microtask, but
 * a backoff flush is a *delayed* timer, which the harness parks in `timers`. The
 * tests fire those parked timers by hand (`runTimers`) — this is what makes an
 * otherwise time-dependent retry loop fully deterministic with no real clock.
 */

import { describe, it, expect, beforeEach } from 'vitest'
import { SyncEngine, type EngineDeps } from '../engine'
import { MemoryStore } from '../queue'
import { peggingState, snapshotFor } from '../__fixtures__/state'
import { FakeWs } from '../__fixtures__/ws'
import { FakeTransport } from '../__fixtures__/transport'
import { backoffDelayMs } from '../backoff'

const MY_USER = 9
const GAME = 1

function makeDeps(transport: FakeTransport, ws: FakeWs) {
  // Delayed timers are parked here so tests can fire backoff flushes explicitly.
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

async function tick() {
  await Promise.resolve()
  await Promise.resolve()
  await Promise.resolve()
}

/** Fire every currently-parked delayed timer once (a manual "clock advance"). */
function runTimers(timers: Array<{ fn: () => void; ms: number }>): void {
  const due = timers.splice(0, timers.length)
  for (const t of due) t.fn()
}

describe('integration: offline replay', () => {
  let ws: FakeWs
  let transport: FakeTransport

  beforeEach(() => {
    ws = new FakeWs()
    transport = new FakeTransport(snapshotFor(peggingState(), MY_USER))
  })

  it('keeps an action pending and schedules a backoff flush on transient failure', async () => {
    transport.failMoveWith(new Error('network down'))

    const { deps, timers } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    engine.dispatch({ kind: 'play_card', card: '5H' })
    await tick()

    // One failed attempt: still pending (not rejected), and a backoff timer parked.
    const item = engine.getState().queue[0]
    expect(item.status).toBe('pending')
    expect(item.attempts).toBe(1)
    expect(transport.moveGameCalls).toHaveLength(1)
    expect(timers).toHaveLength(1)
    expect(timers[0].ms).toBe(backoffDelayMs(1))
    engine.stop()
  })

  it('retries with growing backoff while the network stays down', async () => {
    transport.failMoveWith(new Error('still down'))

    const { deps, timers } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    engine.dispatch({ kind: 'play_card', card: '5H' })
    await tick()
    expect(timers[0].ms).toBe(backoffDelayMs(1))

    // Fire the first backoff timer → second attempt fails → longer backoff parked.
    runTimers(timers)
    await tick()
    expect(transport.moveGameCalls).toHaveLength(2)
    expect(engine.getState().queue[0].attempts).toBe(2)
    expect(timers[0].ms).toBe(backoffDelayMs(2))
    engine.stop()
  })

  it('flushes the pending action once the network recovers', async () => {
    transport.failMoveWith(new Error('transient'))

    const { deps, timers } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    engine.dispatch({ kind: 'play_card', card: '5H' })
    await tick()
    expect(engine.getState().queue[0].status).toBe('pending')

    // Network comes back before the backoff timer fires.
    transport.clearMoveFailure()
    runTimers(timers)
    await tick()

    // Delivered on the retry; the post-flush reconcile drains the queue.
    expect(transport.moveGameCalls.length).toBeGreaterThanOrEqual(2)
    expect(engine.getState().queue.filter((a) => a.status === 'pending' || a.status === 'inflight')).toHaveLength(0)
    engine.stop()
  })

  it('preserves author order when replaying several queued-while-offline actions', async () => {
    transport.failMoveWith(new Error('offline'))

    const { deps, timers } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    // Enqueue two plays while offline. flush() stops at the first transient
    // failure, so only the first has been attempted so far.
    engine.dispatch({ kind: 'play_card', card: '5H' })
    engine.dispatch({ kind: 'play_card', card: '6H' })
    await tick()
    expect(transport.moveGameCalls.map((c) => c.move)).toEqual([{ type: 'play_card', card: '5H' }])

    // Recover and drain: both go out, oldest first.
    transport.clearMoveFailure()
    runTimers(timers)
    await tick()

    expect(transport.moveGameCalls.map((c) => c.move)).toEqual([
      { type: 'play_card', card: '5H' }, // failed attempt
      { type: 'play_card', card: '5H' }, // retry
      { type: 'play_card', card: '6H' },
    ])
    engine.stop()
  })
})
