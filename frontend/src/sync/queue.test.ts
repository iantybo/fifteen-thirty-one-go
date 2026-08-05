import { describe, it, expect, beforeEach } from 'vitest'
import { ActionQueue, MemoryStore, backoffDelayMs, SYNC_QUEUE_PREFIX } from './queue'
import type { SyncAction } from './types'

const GAME = 42
const KEY = `${SYNC_QUEUE_PREFIX}${GAME}`
const play: SyncAction = { kind: 'play_card', card: '5H' }
const go: SyncAction = { kind: 'go' }

describe('ActionQueue — basic operations', () => {
  let store: MemoryStore
  let q: ActionQueue

  beforeEach(() => {
    store = new MemoryStore()
    q = new ActionQueue(GAME, store)
  })

  it('enqueues in author order', () => {
    q.enqueue('a', play, 0, 100)
    q.enqueue('b', go, 0, 101)
    expect(q.list().map((x) => x.clientId)).toEqual(['a', 'b'])
    expect(q.list()[0].status).toBe('pending')
  })

  it('is idempotent for duplicate client ids', () => {
    q.enqueue('a', play, 0, 100)
    q.enqueue('a', go, 1, 200) // same id — ignored
    expect(q.size()).toBe(1)
    expect(q.get('a')?.action).toEqual(play)
  })

  it('transitions status and records attempts', () => {
    q.enqueue('a', play, 0, 100)
    q.setStatus('a', 'inflight')
    expect(q.get('a')?.status).toBe('inflight')
    q.recordAttempt('a')
    q.recordAttempt('a')
    expect(q.get('a')?.attempts).toBe(2)
  })

  it('removes confirmed actions', () => {
    q.enqueue('a', play, 0, 100)
    q.enqueue('b', go, 0, 101)
    q.remove(['a'])
    expect(q.list().map((x) => x.clientId)).toEqual(['b'])
  })

  it('lists only outstanding actions', () => {
    q.enqueue('a', play, 0, 100)
    q.enqueue('b', go, 0, 101)
    q.setStatus('a', 'confirmed')
    expect(q.outstanding().map((x) => x.clientId)).toEqual(['b'])
  })

  it('requeues inflight actions back to pending', () => {
    q.enqueue('a', play, 0, 100)
    q.setStatus('a', 'inflight')
    q.requeueInflight()
    expect(q.get('a')?.status).toBe('pending')
  })

  it('returns defensive copies (mutating a result does not affect the queue)', () => {
    q.enqueue('a', play, 0, 100)
    const copy = q.get('a')!
    copy.status = 'rejected'
    expect(q.get('a')?.status).toBe('pending')
  })
})

describe('ActionQueue — persistence & rehydration', () => {
  it('persists across instances sharing a store', () => {
    const store = new MemoryStore()
    const q1 = new ActionQueue(GAME, store)
    q1.enqueue('a', play, 0, 100)
    q1.enqueue('b', go, 0, 101)

    const q2 = new ActionQueue(GAME, store)
    expect(q2.list().map((x) => x.clientId)).toEqual(['a', 'b'])
  })

  it('continues the sequence counter after rehydration', () => {
    const store = new MemoryStore()
    const q1 = new ActionQueue(GAME, store)
    q1.enqueue('a', play, 0, 100)

    const q2 = new ActionQueue(GAME, store)
    q2.enqueue('b', go, 0, 101)
    // b must sort after a even though it was enqueued by a fresh instance.
    expect(q2.list().map((x) => x.clientId)).toEqual(['a', 'b'])
    expect(q2.list()[1].seq).toBeGreaterThan(q2.list()[0].seq)
  })

  it('drops corrupt persisted payloads instead of throwing', () => {
    const store = new MemoryStore()
    store.setItem(KEY, '{ not valid json')
    const q = new ActionQueue(GAME, store)
    expect(q.size()).toBe(0)
    expect(store.getItem(KEY)).toBeNull() // cleared
  })

  it('filters out structurally-invalid entries', () => {
    const store = new MemoryStore()
    store.setItem(
      KEY,
      JSON.stringify([
        { clientId: 'ok', gameId: GAME, action: play, status: 'pending', baseRevision: 0, seq: 0, createdAt: 1, attempts: 0 },
        { garbage: true },
        { clientId: 123 }, // wrong type
      ]),
    )
    const q = new ActionQueue(GAME, store)
    expect(q.list().map((x) => x.clientId)).toEqual(['ok'])
  })

  it('clear() empties the queue and removes the persisted key', () => {
    const store = new MemoryStore()
    const q = new ActionQueue(GAME, store)
    q.enqueue('a', play, 0, 100)
    q.clear()
    expect(q.size()).toBe(0)
    expect(store.getItem(KEY)).toBeNull()
  })
})

describe('backoffDelayMs', () => {
  it('sends immediately on the first attempt', () => {
    expect(backoffDelayMs(0)).toBe(0)
  })

  it('grows exponentially and caps at 15s', () => {
    expect(backoffDelayMs(1)).toBe(250)
    expect(backoffDelayMs(2)).toBe(500)
    expect(backoffDelayMs(3)).toBe(1000)
    expect(backoffDelayMs(10)).toBe(15_000) // capped
  })
})
