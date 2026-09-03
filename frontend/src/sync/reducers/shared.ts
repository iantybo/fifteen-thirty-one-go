/**
 * Shared helpers for the per-action reducer modules.
 *
 * These are the primitives every action reducer needs: the {@link ApplyResult}
 * contract type, a deep clone, card helpers, and turn advancement. They live in
 * one place so the per-action modules (`play.ts`, `discard.ts`, `go.ts`,
 * `readyNextHand.ts`) stay focused on their own rules.
 */

import type { Card, CribbageState } from '../../api/types'

/** Outcome of applying one action optimistically. */
export type ApplyResult = {
  /** The predicted next state (or the input unchanged when rejected). */
  state: CribbageState
  /** When set, the action could not be applied locally (not an error). */
  rejected?: string
}

/** Pegging value: A=1, 2–10 face value, J/Q/K=10. */
export function cardValue15(c: Card): number {
  return c.rank >= 10 ? 10 : c.rank
}

function rankLabel(rank: number): string {
  return rank === 1 ? 'A' : rank === 11 ? 'J' : rank === 12 ? 'Q' : rank === 13 ? 'K' : String(rank)
}

/** Compact wire code for a card (e.g. "10H", "KS"). */
export function cardToCode(c: Card): string {
  return `${rankLabel(c.rank)}${c.suit}`
}

/** Structural deep clone of a game state (only the arrays reducers mutate). */
export function cloneState(s: CribbageState): CribbageState {
  return {
    ...s,
    cut: s.cut ? { ...s.cut } : undefined,
    hands: s.hands.map((h) => h.map((c) => ({ ...c }))),
    kept_hands: s.kept_hands ? s.kept_hands.map((h) => h.map((c) => ({ ...c }))) : undefined,
    crib: s.crib ? s.crib.map((c) => ({ ...c })) : undefined,
    pegging_seq: s.pegging_seq.map((c) => ({ ...c })),
    pegging_passed: [...s.pegging_passed],
    discard_completed: [...s.discard_completed],
    ready_next_hand: s.ready_next_hand ? [...s.ready_next_hand] : undefined,
    scores: [...s.scores],
  }
}

/**
 * Index of the next player who has not passed, clockwise from `from`. Returns
 * `from` unchanged if everyone else has passed (server adjudicates that case).
 */
export function nextActiveIndex(state: CribbageState, from: number): number {
  const n = state.scores.length
  if (n === 0) return from
  for (let step = 1; step <= n; step++) {
    const idx = (from + step) % n
    if (!state.pegging_passed[idx]) return idx
  }
  return from
}
