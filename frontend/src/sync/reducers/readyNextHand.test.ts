/**
 * Unit tests for the `ready_next_hand` optimistic reducer.
 *
 * The toggle flips my readiness flag during counting, initializing the vector
 * if the snapshot omitted it.
 */

import { describe, it, expect } from 'vitest'
import { applyReadyNextHand } from './readyNextHand'
import { countingState, peggingState } from '../__fixtures__/state'

describe('applyReadyNextHand', () => {
  it('toggles the readiness flag on', () => {
    const state = countingState()
    const res = applyReadyNextHand(state, 0)
    expect(res.rejected).toBeUndefined()
    expect(res.state.ready_next_hand?.[0]).toBe(true)
    // The other player's flag is untouched.
    expect(res.state.ready_next_hand?.[1]).toBe(false)
  })

  it('toggles the readiness flag back off', () => {
    const state = countingState({ ready_next_hand: [true, false] })
    const res = applyReadyNextHand(state, 0)
    expect(res.state.ready_next_hand?.[0]).toBe(false)
  })

  it('initializes an absent readiness vector to all-false first', () => {
    const state = countingState({ ready_next_hand: undefined })
    const res = applyReadyNextHand(state, 1)
    // Vector is created sized to the player count, then my flag is flipped.
    expect(res.state.ready_next_hand).toEqual([false, true])
  })

  it('rejects readying up outside the counting stage', () => {
    const state = peggingState()
    const res = applyReadyNextHand(state, 0)
    expect(res.rejected).toContain('pegging')
    expect(res.state).toBe(state)
  })

  it('does not mutate the input state', () => {
    const state = countingState()
    applyReadyNextHand(state, 0)
    expect(state.ready_next_hand?.[0]).toBe(false)
  })
})
