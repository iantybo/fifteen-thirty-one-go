/**
 * Optimistic reducer — public facade.
 *
 * The per-action rules now live in focused modules under `reducers/`
 * (`play.ts`, `discard.ts`, `go.ts`, `readyNextHand.ts`), each with its own
 * colocated test. This file preserves the original public surface
 * (`applyAction`, `foldActions`, plus the card/state helpers) so existing
 * importers — the engine, the barrel, and tests — keep working after the split.
 *
 * See `reducers/shared.ts` for the shared contract and helpers, and
 * `docs/sync/reducer.md` (if present) / `docs/optimistic-sync-engine.md` for the
 * design narrative: the reducer is pure, total, deterministic, and predicts only
 * locally-derivable state (never scores).
 */

import type { CribbageState } from '../api/types'
import type { SyncAction } from './types'
import {
  type ApplyResult,
  cardToCode,
  cardValue15,
  cloneState,
} from './reducers/shared'
import { applyPlayCard } from './reducers/play'
import { applyDiscard } from './reducers/discard'
import { applyGo } from './reducers/go'
import { applyReadyNextHand } from './reducers/readyNextHand'

// Re-export the action vocabulary and helpers so downstream modules can pull the
// reducer and its primitives from a single import site (backward compatible).
export type { SyncAction, ApplyResult }
export { cardToCode, cardValue15, cloneState }

/**
 * Apply a single optimistic action for the player at `myPos`. Pure and total.
 * `myPos` is the caller's position index within `state.scores`/`state.hands`.
 * Dispatches to the per-action reducer modules.
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
      // Exhaustiveness guard: adding a new SyncAction kind without handling it
      // here is a compile-time error.
      const _never: never = action
      return { state, rejected: `unknown action ${JSON.stringify(_never)}` }
    }
  }
}

/**
 * Fold a list of actions on top of a base state in order, skipping any the
 * reducer rejects (those are left for the server). Returns the final predicted
 * state and the clientIds/reasons that were skipped.
 *
 * The engine calls this to compute `optimisticState` from
 * `confirmedSnapshot.state` + the pending queue, and again to *rebase* the queue
 * after an authoritative snapshot arrives.
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
