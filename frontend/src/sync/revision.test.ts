import { describe, it, expect } from 'vitest'
import { INITIAL_REVISION, isNewer, isStale, bump, max } from './revision'

describe('revision', () => {
  it('starts at 0', () => {
    expect(INITIAL_REVISION).toBe(0)
  })

  it('isNewer is strict greater-than', () => {
    expect(isNewer(2, 1)).toBe(true)
    expect(isNewer(1, 1)).toBe(false)
    expect(isNewer(0, 1)).toBe(false)
  })

  it('isStale is the complement (<=)', () => {
    expect(isStale(1, 1)).toBe(true)
    expect(isStale(0, 1)).toBe(true)
    expect(isStale(2, 1)).toBe(false)
  })

  it('isNewer and isStale partition the space', () => {
    for (const [a, b] of [
      [0, 0],
      [3, 1],
      [1, 3],
      [5, 5],
    ]) {
      expect(isNewer(a, b)).toBe(!isStale(a, b))
    }
  })

  it('bump increments', () => {
    expect(bump(0)).toBe(1)
    expect(bump(41)).toBe(42)
  })

  it('max returns the newer revision', () => {
    expect(max(1, 2)).toBe(2)
    expect(max(9, 3)).toBe(9)
    expect(max(4, 4)).toBe(4)
  })
})
