/**
 * Action fixtures for the sync engine tests.
 *
 * The engine speaks two closely-related vocabularies:
 *
 *  - `SyncAction` — the optimistic action the UI dispatches (see `types.ts`).
 *  - `QueuedAction` — a `SyncAction` wrapped with the durable-queue bookkeeping
 *    (`clientId`, `seq`, `status`, `attempts`, …) that `queue.ts` maintains.
 *
 * These factories keep tests terse and intention-revealing: instead of spelling
 * out a full `QueuedAction` literal (eight required fields, most irrelevant to
 * the assertion at hand) a test writes `queued('c1', playAction('5H'))` and lets
 * the defaults fill in the rest. Every field is overridable so a test can pin
 * exactly the one property it cares about (e.g. `attempts` for backoff cases).
 *
 * Card codes follow the compact form used elsewhere in the fixtures ("5H",
 * "10S", "AC", "KD"); the reducer/`toWireMove` boundary consumes these strings
 * verbatim, so the fixtures deliberately do not parse them into `Card` objects.
 */

import type { QueuedAction, QueuedActionStatus, Revision, SyncAction } from '../types'

/** A `play_card` action. Defaults to the five of hearts used across fixtures. */
export function playAction(card = '5H'): SyncAction {
  return { kind: 'play_card', card }
}

/** A `discard` action. Defaults to discarding the ace and two of hearts. */
export function discardAction(cards: string[] = ['AH', '2H']): SyncAction {
  return { kind: 'discard', cards }
}

/** A `go` action (no payload — the player cannot play without going over 31). */
export function goAction(): SyncAction {
  return { kind: 'go' }
}

/** A `ready_next_hand` action (maps to the dedicated endpoint, not `/move`). */
export function readyAction(): SyncAction {
  return { kind: 'ready_next_hand' }
}

/** Overridable fields when building a `QueuedAction` fixture. */
export type QueuedOverrides = Partial<
  Pick<QueuedAction, 'gameId' | 'status' | 'baseRevision' | 'seq' | 'createdAt' | 'attempts' | 'lastError'>
>

/**
 * Build a `QueuedAction` around a `SyncAction`, filling durable-queue bookkeeping
 * with sensible defaults. `seq` defaults to 0 — tests that care about author
 * ordering should pass explicit ascending `seq` values (that is the invariant
 * `ActionQueue.list()` relies on).
 */
export function queued(clientId: string, action: SyncAction, overrides: QueuedOverrides = {}): QueuedAction {
  const status: QueuedActionStatus = overrides.status ?? 'pending'
  const baseRevision: Revision = overrides.baseRevision ?? 0
  return {
    clientId,
    gameId: overrides.gameId ?? 1,
    action,
    status,
    baseRevision,
    seq: overrides.seq ?? 0,
    createdAt: overrides.createdAt ?? 1000,
    attempts: overrides.attempts ?? 0,
    ...(overrides.lastError !== undefined ? { lastError: overrides.lastError } : {}),
  }
}
