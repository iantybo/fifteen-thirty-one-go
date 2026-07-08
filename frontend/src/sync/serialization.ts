/**
 * Serialization + validation for persisted queue entries.
 *
 * Persistence is a trust boundary: a user can hand-edit `localStorage`, and an
 * older build could have written an incompatible shape. This module owns the
 * (de)serialization and the defensive validation so the queue can stay focused
 * on ordering/status. Malformed entries are dropped, never trusted.
 */

import type { QueuedAction, SyncAction } from './types'

/** Valid `QueuedAction.status` values. */
const STATUSES = new Set(['pending', 'inflight', 'confirmed', 'rejected'])

/** Valid `SyncAction.kind` values. */
const KINDS = new Set(['discard', 'play_card', 'go', 'ready_next_hand'])

/** Type guard for a persisted `SyncAction`. */
export function isSyncAction(v: unknown): v is SyncAction {
  if (!v || typeof v !== 'object') return false
  const a = v as Record<string, unknown>
  if (typeof a.kind !== 'string' || !KINDS.has(a.kind)) return false
  switch (a.kind) {
    case 'discard':
      return Array.isArray(a.cards) && a.cards.every((c) => typeof c === 'string')
    case 'play_card':
      return typeof a.card === 'string'
    case 'go':
    case 'ready_next_hand':
      return true
    default:
      return false
  }
}

/** Type guard for a persisted `QueuedAction`. */
export function isQueuedAction(v: unknown): v is QueuedAction {
  if (!v || typeof v !== 'object') return false
  const a = v as Record<string, unknown>
  return (
    typeof a.clientId === 'string' &&
    typeof a.gameId === 'number' &&
    typeof a.seq === 'number' &&
    typeof a.baseRevision === 'number' &&
    typeof a.createdAt === 'number' &&
    typeof a.attempts === 'number' &&
    typeof a.status === 'string' &&
    STATUSES.has(a.status) &&
    isSyncAction(a.action)
  )
}

/**
 * Parse a persisted JSON string into a validated array of queued actions.
 * Returns `[]` on any parse error or non-array payload, and silently drops
 * individual malformed entries. Never throws.
 */
export function deserializeQueue(raw: string | null): QueuedAction[] {
  if (!raw) return []
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return []
  }
  if (!Array.isArray(parsed)) return []
  return parsed.filter(isQueuedAction)
}

/** Serialize a queue to a JSON string for persistence. */
export function serializeQueue(items: QueuedAction[]): string {
  return JSON.stringify(items)
}
