/**
 * Optimistic reducer for the `discard` action.
 *
 * Extracted from the monolithic reducer so each action's rules live in one
 * small, independently-testable unit. See `../reducer.ts` for the shared
 * contract (pure, total, deterministic) and `docs/sync/reducer.md`.
 */

import type { CribbageState } from '../../api/types'
import type { ApplyResult } from './shared'
import { cloneState, cardToCode } from './shared'

/**
 * Predict the result of the player at `myPos` discarding `cards` during the
 * discard stage. All we can safely predict is:
 *   - the discarded cards leave *my* visible hand, and
 *   - my `discard_completed[myPos]` flips true.
 *
 * We cannot predict the cut card or the transition to pegging because those are
 * server-authored (they depend on the shared deck and the other players).
 */
export function applyDiscard(state: CribbageState, myPos: number, cards: string[]): ApplyResult {
  if (state.stage !== 'discard') {
    return { state, rejected: `cannot discard during stage "${state.stage}"` }
  }
  if (myPos < 0 || myPos >= state.hands.length) {
    return { state, rejected: `unknown player position ${myPos}` }
  }
  const hand = state.hands[myPos]
  if (!hand || hand.length === 0) {
    // We don't have visibility into our own hand — let the server handle it.
    return { state, rejected: 'no local hand to discard from' }
  }
  const toRemove = new Set(cards)
  const remaining = hand.filter((c) => !toRemove.has(cardToCode(c)))
  // If we didn't remove exactly as many distinct cards as were requested, the
  // discard names a card we can't see in hand — leave it for the server.
  if (hand.length - remaining.length !== toRemove.size) {
    return { state, rejected: 'discard references cards not in hand' }
  }
  const next = cloneState(state)
  next.hands[myPos] = remaining
  if (myPos < next.discard_completed.length) {
    next.discard_completed[myPos] = true
  }
  return { state: next }
}
