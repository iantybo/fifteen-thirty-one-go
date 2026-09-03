/**
 * Integration: reconnect + invalidation.
 *
 * Covers what happens when the transport link flaps. The engine's contract on
 * reconnect (`engine.ts#start` → `ws_open`) is:
 *
 *  1. mark connected,
 *  2. `queue.requeueInflight()` — anything mid-flight when the socket dropped has
 *     an unknown fate, so it is reset to `pending` and re-offered (the server
 *     dedupes by `clientId`), and
 *  3. resync (refetch the authoritative snapshot and reconcile).
 *
 * Separately, a bare `game_update` from the server carries no trusted payload,
 * so the engine treats it purely as an invalidation signal and pulls a fresh
 * snapshot over HTTP.
 *
 * As with the other integration suites, determinism comes from the injected
 * `now`/`setTimer` and the `FakeTransport` call recorder — no real sockets, no
 * real clock.
 */

import { describe, it, expect, beforeEach } from 'vitest'
import { SyncEngine, type EngineDeps } from '../engine'
import { MemoryStore } from '../queue'
import { peggingState, snapshotFor } from '../__fixtures__/state'
import { FakeWs } from '../__fixtures__/ws'
import { FakeTransport } from '../__fixtures__/transport'

const MY_USER = 9
const GAME = 1

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

async function tick() {
  await Promise.resolve()
  await Promise.resolve()
  await Promise.resolve()
}

describe('integration: reconnect', () => {
  let ws: FakeWs
  let transport: FakeTransport

  beforeEach(() => {
    ws = new FakeWs()
    transport = new FakeTransport(snapshotFor(peggingState(), MY_USER))
  })

  it('tracks connected=false on ws_close and connected=true on ws_open', async () => {
    const { deps } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    ws.fire('ws_close')
    expect(engine.getState().connected).toBe(false)

    ws.fire('ws_open')
    await tick()
    expect(engine.getState().connected).toBe(true)
    engine.stop()
  })

  it('requeues an inflight action across a close/open and re-sends it', async () => {
    // Hold the first delivery inflight so the socket "drops" mid-flight.
    let release!: () => void
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    let firstCall = true
    transport.moveGame = async (gameId: number, move) => {
      transport.moveGameCalls.push({ gameId, move })
      if (firstCall) {
        firstCall = false
        await gate // never resolves before the drop; simulates a lost request
        return {}
      }
      return {}
    }

    const { deps } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    engine.dispatch({ kind: 'play_card', card: '5H' })
    await tick()
    // Mid-flight: exactly one delivery attempt, still marked inflight.
    expect(transport.moveGameCalls).toHaveLength(1)
    expect(engine.getState().queue[0].status).toBe('inflight')

    // Socket drops then comes back: requeueInflight resets it to pending and the
    // reconnect resync + flush re-offers it to the server.
    ws.fire('ws_close')
    release() // let the stale in-flight promise settle harmlessly
    ws.fire('ws_open')
    await tick()

    // The action was delivered a second time after the reconnect.
    expect(transport.moveGameCalls.length).toBeGreaterThanOrEqual(2)
    engine.stop()
  })

  it('resets inflight actions to pending on ws_open (requeueInflight)', async () => {
    // Block moveGame indefinitely so the action is stuck inflight, then reconnect.
    transport.moveGame = async (gameId: number, move) => {
      transport.moveGameCalls.push({ gameId, move })
      return new Promise<never>(() => {})
    }

    const { deps } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    engine.dispatch({ kind: 'play_card', card: '5H' })
    await tick()
    expect(engine.getState().queue[0].status).toBe('inflight')

    ws.fire('ws_open')
    // requeueInflight runs synchronously inside the ws_open handler, before the
    // async resync — so the status flips back to pending immediately.
    expect(engine.getState().queue[0].status).toBe('pending')
    engine.stop()
  })

  it('refetches the snapshot when a game_update fires', async () => {
    const { deps } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()

    const before = transport.getGameCalls.length
    ws.fire('game_update')
    await tick()

    // game_update is a pure invalidation signal → exactly one extra getGame.
    expect(transport.getGameCalls.length).toBe(before + 1)
    engine.stop()
  })

  it('reconciles to the newer server state delivered via game_update', async () => {
    // Advance the server view, then signal via game_update; the board follows.
    const { deps } = makeDeps(transport, ws)
    const engine = new SyncEngine(GAME, MY_USER, deps)
    await engine.start()
    await tick()
    expect(engine.getState().optimisticState?.pegging_total).toBe(10)

    transport.snapshot = snapshotFor(peggingState({ pegging_total: 15, hands: [[], []], current_index: 1 }), MY_USER)
    ws.fire('game_update')
    await tick()

    expect(engine.getState().optimisticState?.pegging_total).toBe(15)
    expect(engine.getState().revision).toBeGreaterThan(0)
    engine.stop()
  })
})
