import { describe, it, expect } from 'vitest'
import { RealTransport, type GameTransport } from './transport'
import type { GameMoveRequest } from '../api/client'
import type { GameSnapshot } from '../api/types'
import { peggingState, snapshotFor } from './__fixtures__/state'

/**
 * A hand-written fake transport that records calls, demonstrating how cheaply
 * the engine's network dependency can be stubbed once it depends on the
 * {@link GameTransport} interface rather than the concrete `api`.
 */
class FakeTransport implements GameTransport {
  calls: Array<{ method: string; args: unknown[] }> = []
  snapshot: GameSnapshot = snapshotFor(peggingState())

  getGame(gameId: number): Promise<GameSnapshot> {
    this.calls.push({ method: 'getGame', args: [gameId] })
    return Promise.resolve(this.snapshot)
  }

  moveGame(gameId: number, move: GameMoveRequest): Promise<unknown> {
    this.calls.push({ method: 'moveGame', args: [gameId, move] })
    return Promise.resolve({ ok: true })
  }

  nextHand(gameId: number): Promise<void> {
    this.calls.push({ method: 'nextHand', args: [gameId] })
    return Promise.resolve()
  }
}

describe('GameTransport fake', () => {
  it('records getGame calls and returns the configured snapshot', async () => {
    const t = new FakeTransport()
    const snap = await t.getGame(42)
    expect(snap).toBe(t.snapshot)
    expect(t.calls).toEqual([{ method: 'getGame', args: [42] }])
  })

  it('records moveGame calls with the move payload', async () => {
    const t = new FakeTransport()
    const move: GameMoveRequest = { type: 'play_card', card: '5H' }
    await t.moveGame(7, move)
    expect(t.calls).toEqual([{ method: 'moveGame', args: [7, move] }])
  })

  it('records nextHand calls', async () => {
    const t = new FakeTransport()
    await t.nextHand(3)
    expect(t.calls).toEqual([{ method: 'nextHand', args: [3] }])
  })

  it('preserves call order across mixed operations', async () => {
    const t = new FakeTransport()
    await t.getGame(1)
    await t.moveGame(1, { type: 'go' })
    await t.nextHand(1)
    expect(t.calls.map((c) => c.method)).toEqual(['getGame', 'moveGame', 'nextHand'])
  })
})

describe('RealTransport', () => {
  it('implements the GameTransport method surface', () => {
    const t = new RealTransport()
    expect(typeof t.getGame).toBe('function')
    expect(typeof t.moveGame).toBe('function')
    expect(typeof t.nextHand).toBe('function')
  })

  it('is assignable to GameTransport', () => {
    const t: GameTransport = new RealTransport()
    expect(t).toBeInstanceOf(RealTransport)
  })
})
