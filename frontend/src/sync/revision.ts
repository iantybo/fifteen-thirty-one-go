/**
 * Revision arithmetic for the Optimistic Sync Engine.
 *
 * A {@link Revision} is a monotonically non-decreasing integer that identifies a
 * point in a game's authoritative history. The client only ever *compares*
 * revisions — it never interprets the absolute value — so this module is a thin,
 * well-tested wrapper around integer comparison that gives the comparison a
 * name and a single place to change the ordering semantics.
 *
 * See `docs/sync/reconciliation.md` for how revisions gate stale-snapshot
 * rejection.
 */

import type { Revision } from './types'

/** The revision assigned to a freshly-fetched snapshot before any reconcile. */
export const INITIAL_REVISION: Revision = 0

/** True when `incoming` represents strictly newer authoritative state. */
export function isNewer(incoming: Revision, current: Revision): boolean {
  return incoming > current
}

/** True when `incoming` is older-or-equal and should be treated as stale. */
export function isStale(incoming: Revision, current: Revision): boolean {
  return incoming <= current
}

/** Advance a revision by one (used when a plain refetch has no server revision). */
export function bump(current: Revision): Revision {
  return current + 1
}

/** Pick the newer of two revisions. */
export function max(a: Revision, b: Revision): Revision {
  return a > b ? a : b
}
