import { describe, it, expect } from 'vitest'
import { backoffDelayMs, cumulativeBackoffMs, BACKOFF_BASE_MS, BACKOFF_CAP_MS } from './backoff'

describe('backoffDelayMs', () => {
  it('sends immediately on the first attempt', () => {
    expect(backoffDelayMs(0)).toBe(0)
    expect(backoffDelayMs(-1)).toBe(0)
  })

  it('doubles from the base', () => {
    expect(backoffDelayMs(1)).toBe(BACKOFF_BASE_MS)
    expect(backoffDelayMs(2)).toBe(BACKOFF_BASE_MS * 2)
    expect(backoffDelayMs(3)).toBe(BACKOFF_BASE_MS * 4)
  })

  it('caps at the maximum', () => {
    expect(backoffDelayMs(100)).toBe(BACKOFF_CAP_MS)
  })

  it('never exceeds the cap for any input', () => {
    for (let i = 0; i < 50; i++) {
      expect(backoffDelayMs(i)).toBeLessThanOrEqual(BACKOFF_CAP_MS)
    }
  })
})

describe('cumulativeBackoffMs', () => {
  it('is zero for no retries', () => {
    expect(cumulativeBackoffMs(0)).toBe(0)
  })

  it('sums the schedule', () => {
    // 250 + 500 + 1000 = 1750
    expect(cumulativeBackoffMs(3)).toBe(1750)
  })

  it('is monotonically non-decreasing', () => {
    let prev = 0
    for (let i = 0; i <= 20; i++) {
      const cur = cumulativeBackoffMs(i)
      expect(cur).toBeGreaterThanOrEqual(prev)
      prev = cur
    }
  })
})
