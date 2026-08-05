import { describe, it, expect } from 'vitest'
import { isSyncAction, isQueuedAction, deserializeQueue, serializeQueue } from './serialization'
import type { QueuedAction } from './types'

const validAction: QueuedAction = {
  clientId: 'c:1:9:1',
  gameId: 1,
  action: { kind: 'play_card', card: '5H' },
  status: 'pending',
  baseRevision: 0,
  seq: 0,
  createdAt: 123,
  attempts: 0,
}

describe('isSyncAction', () => {
  it('accepts each valid kind', () => {
    expect(isSyncAction({ kind: 'go' })).toBe(true)
    expect(isSyncAction({ kind: 'ready_next_hand' })).toBe(true)
    expect(isSyncAction({ kind: 'play_card', card: '5H' })).toBe(true)
    expect(isSyncAction({ kind: 'discard', cards: ['AH', '2H'] })).toBe(true)
  })

  it('rejects malformed actions', () => {
    expect(isSyncAction(null)).toBe(false)
    expect(isSyncAction({ kind: 'nope' })).toBe(false)
    expect(isSyncAction({ kind: 'play_card' })).toBe(false) // missing card
    expect(isSyncAction({ kind: 'discard', cards: [1, 2] })).toBe(false) // wrong element type
  })
})

describe('isQueuedAction', () => {
  it('accepts a well-formed entry', () => {
    expect(isQueuedAction(validAction)).toBe(true)
  })

  it('rejects entries with missing or wrong-typed fields', () => {
    expect(isQueuedAction({ ...validAction, clientId: 123 })).toBe(false)
    expect(isQueuedAction({ ...validAction, status: 'bogus' })).toBe(false)
    expect(isQueuedAction({ ...validAction, action: { kind: 'nope' } })).toBe(false)
    expect(isQueuedAction({})).toBe(false)
  })
})

describe('deserializeQueue', () => {
  it('returns [] for null/empty/invalid input', () => {
    expect(deserializeQueue(null)).toEqual([])
    expect(deserializeQueue('')).toEqual([])
    expect(deserializeQueue('{not json')).toEqual([])
    expect(deserializeQueue('{"not":"array"}')).toEqual([])
  })

  it('drops malformed entries but keeps valid ones', () => {
    const raw = JSON.stringify([validAction, { garbage: true }, { ...validAction, status: 'x' }])
    const out = deserializeQueue(raw)
    expect(out).toHaveLength(1)
    expect(out[0].clientId).toBe('c:1:9:1')
  })

  it('round-trips through serialize', () => {
    const raw = serializeQueue([validAction])
    expect(deserializeQueue(raw)).toEqual([validAction])
  })
})
