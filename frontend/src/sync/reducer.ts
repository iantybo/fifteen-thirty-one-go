/**
 * Optimistic reducer for the sync engine.
 *
 * This is the heart of the "prediction" half of optimistic sync. Given the last
 * authoritative `CribbageState` and a `SyncAction`, it produces the state the
 * player *expects* to see immediately — before the server has confirmed
 * anything.
 *
 * ## Design rules
 *
 * 1. **Pure and total.** `applyAction` never mutates its input and never
 *    throws. If an action cannot be applied optimistically (because we lack the
 *    information the server has), it returns the state unchanged plus a
 *    `rejected` reason. This keeps the engine simple: an un-appliable action is
 *    just left for the server to adjudicate.
 *
 * 2. **Conservative.** The client does not know the full deck, the opponent's
 *    hand, or the exact scoring the server will award. The reducer therefore
 *    only predicts the parts of state that are *locally derivable* — whose turn
 *    it is, the pegging total, which of my cards left my hand — and leaves
 *    scores for the server to reconcile. Predicting a score we can't verify
 *    would guarantee a visible rollback, which is worse than a brief "pending".
 *
 * 3. **Deterministic.** No `Date.now()`, no randomness. Same inputs → same
 *    output. This is what makes the reducer unit-testable and what lets the
 *    engine re-fold the queue on top of any base snapshot.
 *
 * The reducer mirrors the semantics enforced authoritatively in
 * `backend/internal/game/cribbage/cribbage.go`; where the two disagree the
 * server always wins during reconciliation.
 */

import type { Card, CribbageState } from '../api/types'
import type { SyncAction } from './types'

// Re-export so tests and downstream modules can pull the action vocabulary and
// the reducer from a single import site.
export type { SyncAction }

/** Outcome of applying one action optimistically. */
export type ApplyResult = {
  /** The predicted next state (or the input unchanged when rejected). */
  state: CribbageState
  /**
   * When set, the action could not be applied locally and should be sent to the
   * server without an optimistic preview. Not an error — just "we don't know".
   */
  rejected?: string
}

/** Pegging value: A=1, 2–10 face value, J/Q/K=10. Mirrors GamePage#cardValue15. */
export function cardValue15(c: Card): number {
  return c.rank >= 10 ? 10 : c.rank
}

/** Card rank letter used in the compact wire code (e.g. "10H", "KS"). */
function rankLabel(rank: number): string {
  return rank === 1 ? 'A' : rank === 11 ? 'J' : rank === 12 ? 'Q' : rank === 13 ? 'K' : String(rank)
}

/** Compact wire code for a card, matching the backend's card serialization. */
export function cardToCode(c: Card): string {
  return `${rankLabel(c.rank)}${c.suit}`
}

/**
 * Structurally clone a `CribbageState`. We hand-roll this rather than reaching
 * for `structuredClone` so it works in every test runtime and so we can be
 * explicit about which nested arrays are copied (the ones the reducer mutates).
 */
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
 * Find the index of the next player who has not passed, moving clockwise from
 * `from`. Returns `from` unchanged if everyone else has passed (the engine
 * leaves the "GO"/reset adjudication to the server in that case).
 */
function nextActiveIndex(state: CribbageState, from: number): number {
  const n = state.scores.length
  if (n === 0) return from
  for (let step = 1; step <= n; step++) {
    const idx = (from + step) % n
    if (!state.pegging_passed[idx]) return idx
  }
  return from
}

/**
 * Apply a `discard` optimistically. During the discard stage all we can safely
 * predict is:
 *   - the discarded cards leave *my* visible hand, and
 *   - my `discard_completed[myPos]` flips true.
 * We cannot predict the cut card or the transition to pegging because those are
 * server-authored (they depend on the shared deck and the other players).
 */
function applyDiscard(state: CribbageState, myPos: number, cards: string[]): ApplyResult {
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

/**
 * Apply a `play_card` optimistically during pegging:
 *   - remove the card from my hand,
 *   - push it onto the shared pegging sequence,
 *   - add its 15-value to the running total,
 *   - advance the turn to the next active player.
 *
 * Pegging *scores* (fifteens, pairs, runs, thirty-ones) are intentionally NOT
 * predicted here — they are subtle and the server is the arbiter. The peg board
 * will jump forward on reconciliation, which reads as "the server awarded
 * points", exactly the mental model we want.
 */
function applyPlayCard(state: CribbageState, myPos: number, code: string): ApplyResult {
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

/**
 * Apply a `go` optimistically: mark me as passed and advance the turn. The
 * server owns the "everyone passed → reset the count / award the go point"
 * transition, so we do not touch the pegging total here.
 */
function applyGo(state: CribbageState, myPos: number): ApplyResult {
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

/**
 * Apply a `ready_next_hand` toggle optimistically during counting: flip my
 * readiness flag. The actual deal of the next hand is server-authored.
 */
function applyReadyNextHand(state: CribbageState, myPos: number): ApplyResult {
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

/**
 * Apply a single optimistic action for the player at `myPos`. Pure and total —
 * see the module doc for the contract. `myPos` is the caller's position index
 * within `state.scores`/`state.hands`.
 */
export function applyAction(state: CribbageState, action: SyncAction, myPos: number): ApplyResult {
  switch (action.kind) {
    case 'discard':
      return applyDiscard(state, myPos, action.cards)
    case 'play_card':
      return applyPlayCard(state, myPos, action.card)
    case 'go':
      return applyGo(state, myPos)
    case 'ready_next_hand':
      return applyReadyNextHand(state, myPos)
    default: {
      // Exhaustiveness guard: if a new action kind is added to SyncAction and
      // not handled here, TypeScript flags this line at compile time.
      const _never: never = action
      return { state, rejected: `unknown action ${JSON.stringify(_never)}` }
    }
  }
}

/**
 * Fold a list of actions on top of a base state in order, skipping any that the
 * reducer rejects (those are left for the server). Returns both the final
 * predicted state and the clientIds/reasons that were skipped so the engine can
 * decide whether to send them without an optimistic preview.
 *
 * This is what the engine calls to compute `optimisticState` from
 * `confirmedSnapshot.state` + the pending queue, and what it calls again to
 * *rebase* the queue after an authoritative snapshot arrives.
 */
export function foldActions(
  base: CribbageState,
  actions: Array<{ clientId: string; action: SyncAction }>,
  myPos: number,
): { state: CribbageState; skipped: Array<{ clientId: string; reason: string }> } {
  let state = base
  const skipped: Array<{ clientId: string; reason: string }> = []
  for (const { clientId, action } of actions) {
    const res = applyAction(state, action, myPos)
    if (res.rejected) {
      skipped.push({ clientId, reason: res.rejected })
      continue
    }
    state = res.state
  }
  return { state, skipped }
}
