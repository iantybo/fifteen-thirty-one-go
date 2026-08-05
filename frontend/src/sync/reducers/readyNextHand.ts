/**
 * Optimistic reducer for the `ready_next_hand` action.
 *
 * Extracted from the monolithic reducer so each action's rules live in one
 * small, independently-testable unit. See `../reducer.ts` for the shared
 * contract (pure, total, deterministic) and `docs/sync/reducer.md`.
 */

import type { CribbageState } from '../../api/types'
import type { ApplyResult } from './shared'
import { cloneState } from './shared'

/**
 * Predict the result of the player at `myPos` toggling their readiness during
 * the counting stage: flip my `ready_next_hand[myPos]` flag.
 *
 * The readiness vector may be absent on a fresh counting snapshot, so we
 * initialize it to all-false first. The actual deal of the next hand is
 * server-authored — we only mirror the local toggle.
 */
export function applyReadyNextHand(state: CribbageState, myPos: number): ApplyResult {
  if (state.stage !== 'counting') {
    return { state, rejected: `cannot ready up during stage "${state.stage}"` }
  }
  const next = cloneState(state)
  if (!next.ready_next_hand) {
    next.ready_next_hand = new Array(state.scores.length).fill(false)
  }
  if (myPos >= 0 && myPos < next.ready_next_hand.length) {
    next.ready_next_hand[myPos] = !next.ready_next_hand[myPos]
  }
  return { state: next }
}
