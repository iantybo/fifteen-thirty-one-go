/**
 * Retry backoff schedule for action delivery.
 *
 * The schedule is a capped exponential with a deterministic (non-random) delay
 * so tests can assert exact values and so two clients retrying the same failed
 * action stay in lockstep rather than thundering. Randomized jitter is
 * intentionally omitted here; if we ever need it, it belongs in a separate
 * decorator so the base schedule stays pure and testable.
 */

/** Base delay for the first retry, in milliseconds. */
export const BACKOFF_BASE_MS = 250

/** Maximum delay any retry will wait, in milliseconds. */
export const BACKOFF_CAP_MS = 15_000

/**
 * Delay before the next attempt given the number of attempts already made.
 *
 * - `attempts <= 0` → 0 (send immediately)
 * - `attempts = 1`  → BACKOFF_BASE_MS
 * - each subsequent attempt doubles, capped at BACKOFF_CAP_MS
 */
export function backoffDelayMs(attempts: number): number {
  if (attempts <= 0) return 0
  return Math.min(BACKOFF_CAP_MS, BACKOFF_BASE_MS * 2 ** (attempts - 1))
}

/** Total time spent waiting across `n` retries (useful for give-up policies). */
export function cumulativeBackoffMs(n: number): number {
  let total = 0
  for (let i = 1; i <= n; i++) total += backoffDelayMs(i)
  return total
}
