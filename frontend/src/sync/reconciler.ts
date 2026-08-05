/**
 * Pure reconciliation planning for the Optimistic Sync Engine.
 *
 * ## Why this module exists
 *
 * `engine.ts#reconcile` mixes three concerns: mutating the durable queue,
 * re-folding the reducer, and — buried in the middle — *deciding* what should
 * happen given an incoming authoritative snapshot. That decision is a pure
 * function of a handful of scalars (the two revisions, whether we have ever
 * confirmed a snapshot, and the three id lists), and it is the part most worth
 * testing in isolation because it encodes the subtle stale-snapshot rule that
 * keeps the board from jumping backward on out-of-order deliveries.
 *
 * This module extracts exactly that decision as `planReconcile`. It performs no
 * I/O, touches no queue, and folds no reducer: it takes the reconcile *inputs*
 * and returns the {@link ReconcileResult} the engine should apply. The engine
 * could adopt it wholesale by feeding it the current revision + the ids it
 * already computes; keeping it standalone means the reconciliation policy can be
 * unit-tested and reasoned about without spinning up a full engine, a queue, a
 * WebSocket, and a fake clock.
 *
 * The ordering semantics (what counts as "newer" vs "stale") are delegated to
 * `revision.ts` so there is a single source of truth for revision arithmetic.
 *
 * See `docs/sync/reconciliation.md` for the design narrative behind the
 * stale-snapshot gate and the confirm/reject/keep partition.
 */

import type { ReconcileResult, Revision } from './types'
import { isNewer, isStale } from './revision'

/**
 * Everything `planReconcile` needs to decide a reconciliation outcome, with no
 * dependency on the engine, queue, or transport.
 *
 * All three id lists are `clientId`s. `outstanding` is the set of actions still
 * in the queue (pending or inflight) *before* this reconcile; `accepted` and
 * `rejected` are the server's verdicts on some subset of them.
 */
export type ReconcileInput = {
  /** The engine's current (locally-known) revision. */
  currentRevision: Revision
  /** The revision carried by the incoming authoritative snapshot. */
  incomingRevision: Revision
  /**
   * Whether the engine has ever accepted an authoritative snapshot. On the very
   * first load this is `false`, which suppresses the stale-snapshot gate so the
   * initial snapshot is never mistaken for a late/duplicate delivery.
   */
  hasConfirmed: boolean
  /** clientIds of actions still outstanding before this reconcile. */
  outstanding: string[]
  /** clientIds the server confirmed. */
  accepted: string[]
  /** clientIds the server rejected. */
  rejected: string[]
}

/**
 * Decide how to reconcile local optimistic state against an authoritative
 * snapshot, as a pure function.
 *
 * Rules (mirrors `engine.ts#reconcile`):
 *
 *  - **Stale gate.** If the incoming revision is not strictly newer than the
 *    current one *and* we have already confirmed at least one snapshot, the
 *    snapshot is a late/duplicate delivery. We ignore it: `ignoredStale` is
 *    `true`, everything stays pending (`keptPending = outstanding`), nothing is
 *    confirmed or rejected, and the revision does not move. This is what stops
 *    the board from regressing on out-of-order WS deliveries.
 *
 *    The `hasConfirmed` guard is essential: on the *first* snapshot there is no
 *    prior authoritative state to protect, so even a non-advancing revision
 *    (e.g. both `0`) must be accepted rather than ignored.
 *
 *  - **Advance.** Otherwise the snapshot is authoritative: `confirmed` becomes
 *    the server's `accepted` list, `rejected` is passed through, and everything
 *    still outstanding that the server did *not* rule on stays pending
 *    (`keptPending = outstanding − (accepted ∪ rejected)`). `ignoredStale` is
 *    `false` and the revision advances to `incomingRevision`.
 *
 * @see docs/sync/reconciliation.md
 */
export function planReconcile(input: ReconcileInput): ReconcileResult {
  const { currentRevision, incomingRevision, hasConfirmed, outstanding, accepted, rejected } = input

  // Stale/duplicate delivery: ignore it, but only once we have something to
  // protect. `isStale` is the inverse of `isNewer`; we consult both explicitly
  // so the intent reads clearly and the ordering source-of-truth stays in
  // `revision.ts`.
  if (hasConfirmed && isStale(incomingRevision, currentRevision)) {
    return {
      confirmed: [],
      rejected: [],
      keptPending: [...outstanding],
      ignoredStale: true,
      revision: currentRevision,
    }
  }

  // Sanity: reaching here means either it is the first snapshot or the incoming
  // revision is strictly newer. (`isNewer` is referenced to document that
  // invariant; the branch above already handled the stale case.)
  void isNewer

  return {
    confirmed: [...accepted],
    rejected: [...rejected],
    keptPending: dropReconciled(outstanding, accepted, rejected),
    ignoredStale: false,
    revision: incomingRevision,
  }
}

/**
 * Compute the actions that remain pending after a reconcile: everything in
 * `outstanding` that the server neither accepted nor rejected, preserving the
 * original outstanding order.
 *
 * Pure and allocation-only — it never mutates its inputs.
 */
export function dropReconciled(outstanding: string[], accepted: string[], rejected: string[]): string[] {
  const reconciled = new Set<string>([...accepted, ...rejected])
  return outstanding.filter((clientId) => !reconciled.has(clientId))
}
