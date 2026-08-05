/**
 * Error classification for the sync engine's flush loop.
 *
 * The engine must decide, for every failed delivery, whether to **retry**
 * (transient) or **roll back** (permanent). Getting this wrong is costly:
 * retrying a permanent error spins forever; rolling back a transient error
 * loses a legitimate move. This module centralizes that decision so both the
 * engine and its tests agree on the taxonomy.
 */

import { ApiError } from '../lib/http'

/** HTTP statuses that are transient even though they are in the 4xx range. */
export const TRANSIENT_4XX = new Set([408, 425, 429])

/**
 * True when an error should NOT be retried — the request will never succeed as
 * written (illegal move, out of turn, duplicate, validation failure). These are
 * rolled back and surfaced to the user.
 */
export function isPermanentError(e: unknown): boolean {
  if (e instanceof ApiError) {
    if (TRANSIENT_4XX.has(e.status)) return false
    return e.status >= 400 && e.status < 500
  }
  return false
}

/** True when an error is worth retrying with backoff (network blips, 5xx, 429). */
export function isTransientError(e: unknown): boolean {
  if (e instanceof ApiError) {
    if (TRANSIENT_4XX.has(e.status)) return true
    return e.status >= 500
  }
  // Non-API errors (fetch/network/DNS) are assumed transient.
  return true
}

/** Extract a human-readable message from any thrown value. */
export function errorMessage(e: unknown): string {
  if (e instanceof ApiError) return e.message
  if (e instanceof Error) return e.message
  if (typeof e === 'string') return e
  return 'unknown error'
}
