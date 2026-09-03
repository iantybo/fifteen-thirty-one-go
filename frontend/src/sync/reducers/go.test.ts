/**
 * Unit tests for the `go` optimistic reducer.
 *
 * A "GO" marks the current player as passed and advances the turn, but must not
 * touch the pegging total — that reset is server-authored.
 */

import { describe, it, expect } from 'vitest'
import { applyGo } from './go'
import { peggingState, countingState } from '../__fixtures__/state'

describe('applyGo', () => {
  it('marks the player passed and advances the turn', () => {
    const state = peggingState()
    const res = applyGo(state, 0)
    expect(res.rejected).toBeUndefined()
    expect(res.state.pegging_passed[0]).toBe(true)
    // Turn moves to the next active (not-passed) player.
    expect(res.state.current_index).toBe(1)
  })

  it('leaves the pegging total unchanged', () => {
    const state = peggingState({ pegging_total: 21 })
    const res = applyGo(state, 0)
    expect(res.state.pegging_total).toBe(21)
  })

  it('rejects a GO outside the pegging stage', () => {
    const state = countingState()
    const res = applyGo(state, 0)
    expect(res.rejected).toContain('counting')
    expect(res.state).toBe(state)
  })

  it('rejects a GO when it is not your turn', () => {
    const state = peggingState({ current_index: 1 })
    const res = applyGo(state, 0)
    expect(res.rejected).toBe('not your turn')
    expect(res.state).toBe(state)
  })

  it('does not mutate the input state', () => {
    const state = peggingState()
    applyGo(state, 0)
    expect(state.pegging_passed[0]).toBe(false)
    expect(state.current_index).toBe(0)
  })
})
