/**
 * Optimistic reducer for the `play_card` action.
 *
 * Extracted from the monolithic reducer so each action's rules live in one
 * small, independently-testable unit. See `../reducer.ts` for the shared
 * contract (pure, total, deterministic) and `docs/sync/reducer.md`.
 */

import type { CribbageState } from '../../api/types'
import type { ApplyResult } from './shared'
import { cloneState, cardToCode, cardValue15, nextActiveIndex } from './shared'

/**
 * Predict the result of the player at `myPos` playing `code` during pegging:
 * remove the card from hand, push onto the sequence, advance the total and the
 * turn. Pegging *scores* are not predicted — the server is the arbiter.
 */
export function applyPlayCard(state: CribbageState, myPos: number, code: string): ApplyResult {
  if (state.stage !== 'pegging') {
    return { state, rejected: `cannot play a card during stage "${state.stage}"` }
  }
  if (state.current_index !== myPos) {
    return { state, rejected: 'not your turn' }
  }
  const hand = state.hands[myPos]
  if (!hand) return { state, rejected: `unknown player position ${myPos}` }
  const idx = hand.findIndex((c) => cardToCode(c) === code)
  if (idx === -1) {
    return { state, rejected: `card ${code} not in hand` }
  }
  const card = hand[idx]
  const value = cardValue15(card)
  if (state.pegging_total + value > 31) {
    return { state, rejected: `playing ${code} would exceed 31` }
  }
  const next = cloneState(state)
  next.hands[myPos] = [...hand.slice(0, idx), ...hand.slice(idx + 1)]
  next.pegging_seq = [...next.pegging_seq, { ...card }]
  next.pegging_total = state.pegging_total + value
  next.last_play_index = myPos
  next.current_index = nextActiveIndex(next, myPos)
  return { state: next }
}
