import { describe, it, expect } from 'vitest'
import { decideOnError, orderForDelivery, type FlushDecision } from './flusher'
import { ApiError } from '../lib/http'
import { backoffDelayMs } from './backoff'

describe('decideOnError', () => {
  it('rejects a permanent 4xx with the error message as reason', () => {
    const e = new ApiError('illegal move', 422)
    const d = decideOnError(e, 1)
    expect(d.action).toBe('reject')
    expect(d.reason).toBe('illegal move')
    expect(d.delayMs).toBeUndefined()
  })

  it('rejects other 4xx (e.g. 400/403/409) as permanent', () => {
    for (const status of [400, 403, 409]) {
      const d = decideOnError(new ApiError('nope', status), 1)
      expect(d.action).toBe('reject')
    }
  })

  it('retries a 5xx with a backoff delay', () => {
    const e = new ApiError('server boom', 500)
    const d = decideOnError(e, 1)
    expect(d.action).toBe('retry')
    expect(d.delayMs).toBe(backoffDelayMs(1))
  })

  it('retries the transient 4xx statuses (408/425/429)', () => {
    for (const status of [408, 425, 429]) {
      const d = decideOnError(new ApiError('slow down', status), 2)
      expect(d.action).toBe('retry')
      expect(d.delayMs).toBe(backoffDelayMs(2))
    }
  })

  it('retries a non-ApiError (network) failure', () => {
    const d = decideOnError(new TypeError('failed to fetch'), 3)
    expect(d.action).toBe('retry')
    expect(d.delayMs).toBe(backoffDelayMs(3))
    expect(d.reason).toBe('failed to fetch')
  })

  it('scales the retry delay with the attempt count', () => {
    const first = decideOnError(new ApiError('x', 503), 1)
    const later = decideOnError(new ApiError('x', 503), 4)
    expect(later.delayMs!).toBeGreaterThan(first.delayMs!)
  })
})

describe('orderForDelivery', () => {
  it('sorts by seq ascending', () => {
    const items = [
      { seq: 3, clientId: 'c' },
      { seq: 1, clientId: 'a' },
      { seq: 2, clientId: 'b' },
    ]
    expect(orderForDelivery(items).map((i) => i.clientId)).toEqual(['a', 'b', 'c'])
  })

  it('does not mutate the input array', () => {
    const items = [
      { seq: 2, clientId: 'b' },
      { seq: 1, clientId: 'a' },
    ]
    const before = items.map((i) => i.clientId)
    orderForDelivery(items)
    expect(items.map((i) => i.clientId)).toEqual(before)
  })

  it('is stable for equal seq values', () => {
    const items = [
      { seq: 1, clientId: 'first' },
      { seq: 1, clientId: 'second' },
      { seq: 1, clientId: 'third' },
    ]
    expect(orderForDelivery(items).map((i) => i.clientId)).toEqual(['first', 'second', 'third'])
  })

  it('returns a new array instance', () => {
    const items = [{ seq: 1, clientId: 'a' }]
    expect(orderForDelivery(items)).not.toBe(items)
  })

  it('handles the empty case', () => {
    expect(orderForDelivery([])).toEqual([])
  })

  // Type-shape sanity: FlushDecision unions are assignable as documented.
  it('exposes the documented FlushDecision shape', () => {
    const d: FlushDecision = { action: 'send' }
    expect(d.action).toBe('send')
  })
})
