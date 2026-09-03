/**
 * Pure planning helpers for the Optimistic Sync Engine's flush loop.
 *
 * ## Why this module exists
 *
 * `engine.ts#flush` interleaves I/O (POSTing moves, awaiting responses,
 * mutating the queue, scheduling timers) with two small but consequential
 * *decisions*:
 *
 *  1. On a delivery failure, should we **retry** the action (and after how
 *     long) or **reject** it and roll back?
 *  2. In what order should outstanding actions be delivered?
 *
 * Both are pure functions of their inputs, and both are easy to get subtly
 * wrong — retrying a permanent 4xx spins forever; rejecting a transient blip
 * loses a legitimate move; delivering pegging moves out of order corrupts the
 * hand. This module extracts those decisions so they can be tested exhaustively
 * without a network, a queue, or a clock. The engine could adopt them by
 * calling `decideOnError` in its `catch` block and `orderForDelivery` before it
 * iterates.
 *
 * The error taxonomy lives in `errors.ts` and the backoff schedule in
 * `backoff.ts`; this module composes them rather than re-deriving either, so
 * there is a single source of truth for "is this permanent?" and "how long do
 * we wait?".
 */

import { isPermanentError, errorMessage } from './errors'
import { backoffDelayMs } from './backoff'

/**
 * What the flush loop should do with a single action after an attempt failed.
 *
 *  - `send`   — (not produced by `decideOnError`) the action is ready to be
 *               delivered; included so callers can model the full loop.
 *  - `retry`  — transient failure: keep the action pending and re-attempt after
 *               `delayMs`.
 *  - `reject` — permanent failure: roll the action back and surface `reason`.
 */
export type FlushDecision = {
  action: 'send' | 'retry' | 'reject'
  /** Backoff delay before the next attempt (only set for `retry`). */
  delayMs?: number
  /** Human-readable explanation (set for `reject`, and echoed for `retry`). */
  reason?: string
}

/**
 * Classify a delivery failure into a retry-with-backoff or a rollback.
 *
 * `attempts` is the number of delivery attempts already made for this action
 * (including the one that just failed), so `backoffDelayMs(attempts)` yields the
 * delay before the *next* attempt.
 *
 *  - Permanent (illegal move / out of turn / duplicate / validation): the
 *    request will never succeed as written, so we `reject` with the extracted
 *    error message as `reason`.
 *  - Transient (network blip / 5xx / 408 / 425 / 429): worth another try, so we
 *    `retry` after `backoffDelayMs(attempts)`, carrying the message along for
 *    diagnostics.
 *
 * Pure: no I/O, no mutation. Delegates classification to `errors.ts` and timing
 * to `backoff.ts`.
 */
export function decideOnError(e: unknown, attempts: number): FlushDecision {
  const reason = errorMessage(e)
  if (isPermanentError(e)) {
    return { action: 'reject', reason }
  }
  return { action: 'retry', delayMs: backoffDelayMs(attempts), reason }
}

/**
 * Return a copy of `items` sorted by `seq` ascending — the order in which
 * outstanding actions must be delivered to preserve author order (cribbage
 * moves are order-dependent).
 *
 * The sort is stable for equal `seq` values (modern V8 `Array.prototype.sort`
 * is stable), so ties retain their input order. The input array is never
 * mutated.
 */
export function orderForDelivery<T extends { seq: number; clientId: string }>(items: T[]): T[] {
  return [...items].sort((a, b) => a.seq - b.seq)
}
