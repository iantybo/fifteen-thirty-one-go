/**
 * Optimistic reducer for the `go` action.
 *
 * Extracted from the monolithic reducer so each action's rules live in one
 * small, independently-testable unit. See `../reducer.ts` for the shared
 * contract (pure, total, deterministic) and `docs/sync/reducer.md`.
 */

import type { CribbageState } from '../../api/types'
import type { ApplyResult } from './shared'
import { cloneState, nextActiveIndex } from './shared'

/**
 * Predict the result of the player at `myPos` saying "GO" during pegging: mark
 * me as passed and advance the turn to the next active player.
 *
 * The server owns the "everyone passed → reset the count / award the go point"
 * transition, so we deliberately do NOT touch `pegging_total` here.
 */
export function applyGo(state: CribbageState, myPos: number): ApplyResult {
  if (state.stage !== 'pegging') {
    return { state, rejected: `cannot say GO during stage "${state.stage}"` }
  }
  if (state.current_index !== myPos) {
    return { state, rejected: 'not your turn' }
  }
  const next = cloneState(state)
  if (myPos < next.pegging_passed.length) {
    next.pegging_passed[myPos] = true
  }
  next.current_index = nextActiveIndex(next, myPos)
  return { state: next }
}
