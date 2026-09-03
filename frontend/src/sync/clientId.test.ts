import { describe, it, expect } from 'vitest'
import { IdGenerator, parseClientId, isClientId } from './clientId'

describe('IdGenerator', () => {
  it('produces monotonically increasing ids', () => {
    const g = new IdGenerator(1, 9)
    expect(g.next()).toBe('c:1:9:1')
    expect(g.next()).toBe('c:1:9:2')
    expect(g.next()).toBe('c:1:9:3')
    expect(g.current()).toBe(3)
  })

  it('encodes game and user', () => {
    const g = new IdGenerator(42, 7)
    expect(g.next()).toBe('c:42:7:1')
  })

  it('is deterministic (no randomness)', () => {
    const a = new IdGenerator(1, 1)
    const b = new IdGenerator(1, 1)
    expect(a.next()).toBe(b.next())
    expect(a.next()).toBe(b.next())
  })
})

describe('parseClientId', () => {
  it('round-trips a generated id', () => {
    const id = new IdGenerator(3, 5).next()
    expect(parseClientId(id)).toEqual({ gameId: 3, userId: 5, seq: 1 })
  })

  it('returns null for malformed ids', () => {
    expect(parseClientId('')).toBeNull()
    expect(parseClientId('x:1:2:3')).toBeNull()
    expect(parseClientId('c:1:2')).toBeNull()
    expect(parseClientId('c:a:b:c')).toBeNull()
  })
})

describe('isClientId', () => {
  it('recognizes valid and invalid ids', () => {
    expect(isClientId('c:1:9:1')).toBe(true)
    expect(isClientId('nope')).toBe(false)
  })
})
