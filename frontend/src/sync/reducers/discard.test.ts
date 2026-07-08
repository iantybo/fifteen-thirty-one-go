/**
 * Unit tests for the `discard` optimistic reducer.
 *
 * The reducer only predicts the locally-derivable effects of a discard — the
 * cards leaving my visible hand and my completion flag flipping — so these
 * tests focus on exactly those, plus the rejection paths and immutability.
 */

import { describe, it, expect } from 'vitest'
import { applyDiscard } from './discard'
import { discardState, peggingState } from '../__fixtures__/state'

describe('applyDiscard', () => {
  it('removes the discarded cards and marks the player completed', () => {
    const state = discardState()
    const res = applyDiscard(state, 0, ['AH', '2H'])
    expect(res.rejected).toBeUndefined()
    // The two discarded cards are gone; the rest of the hand is intact.
    expect(res.state.hands[0].map((c) => c.rank)).toEqual([3, 4, 5, 6])
    expect(res.state.discard_completed[0]).toBe(true)
    // The other player's completion flag is untouched.
    expect(res.state.discard_completed[1]).toBe(false)
  })

  it('rejects a discard that references a card not in hand', () => {
    const state = discardState()
    const res = applyDiscard(state, 0, ['KS'])
    expect(res.rejected).toBeDefined()
    // On rejection the input is returned unchanged.
    expect(res.state).toBe(state)
  })

  it('rejects a discard outside the discard stage', () => {
    const state = peggingState()
    const res = applyDiscard(state, 0, ['5H'])
    expect(res.rejected).toContain('pegging')
    expect(res.state).toBe(state)
  })

  it('rejects when the local hand is empty (hidden)', () => {
    const state = discardState()
    const res = applyDiscard(state, 1, ['AH', '2H'])
    expect(res.rejected).toBeDefined()
    expect(res.state).toBe(state)
  })

  it('does not mutate the input state', () => {
    const state = discardState()
    const before = state.hands[0].length
    applyDiscard(state, 0, ['AH', '2H'])
    expect(state.hands[0].length).toBe(before)
    expect(state.discard_completed[0]).toBe(false)
  })
})
