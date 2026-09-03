/**
 * Unit tests for the `play_card` optimistic reducer.
 *
 * Playing a card removes it from my hand, appends it to the pegging sequence,
 * bumps the running total, and advances the turn. Pegging *scores* are left to
 * the server, so we do not assert on them here.
 */

import { describe, it, expect } from 'vitest'
import { applyPlayCard } from './play'
import { peggingState, countingState } from '../__fixtures__/state'

describe('applyPlayCard', () => {
  it('removes the card and advances total and turn', () => {
    const state = peggingState()
    const res = applyPlayCard(state, 0, '5H')
    expect(res.rejected).toBeUndefined()
    // 5H is gone from the hand; 6H remains.
    expect(res.state.hands[0].map((c) => c.rank)).toEqual([6])
    // Sequence grew and total advanced by the card's 15-value (5).
    expect(res.state.pegging_seq.at(-1)).toMatchObject({ rank: 5, suit: 'H' })
    expect(res.state.pegging_total).toBe(15)
    expect(res.state.last_play_index).toBe(0)
    expect(res.state.current_index).toBe(1)
  })

  it('rejects a play that would exceed 31', () => {
    const state = peggingState({ pegging_total: 28 })
    const res = applyPlayCard(state, 0, '6H')
    expect(res.rejected).toContain('exceed 31')
    expect(res.state).toBe(state)
  })

  it('rejects a card that is not in hand', () => {
    const state = peggingState()
    const res = applyPlayCard(state, 0, 'KS')
    expect(res.rejected).toContain('not in hand')
    expect(res.state).toBe(state)
  })

  it('rejects a play outside the pegging stage', () => {
    const state = countingState()
    const res = applyPlayCard(state, 0, '5H')
    expect(res.rejected).toContain('counting')
    expect(res.state).toBe(state)
  })

  it('rejects a play when it is not your turn', () => {
    const state = peggingState({ current_index: 1 })
    const res = applyPlayCard(state, 0, '5H')
    expect(res.rejected).toBe('not your turn')
    expect(res.state).toBe(state)
  })

  it('does not mutate the input state', () => {
    const state = peggingState()
    applyPlayCard(state, 0, '5H')
    expect(state.hands[0].length).toBe(2)
    expect(state.pegging_total).toBe(10)
    expect(state.pegging_seq.length).toBe(1)
  })
})
