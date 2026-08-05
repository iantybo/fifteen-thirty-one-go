/**
 * Barrel for the per-action optimistic reducer modules.
 *
 * Each action's rules live in its own file (`play.ts`, `discard.ts`, `go.ts`,
 * `readyNextHand.ts`) so they stay focused and independently testable. This
 * module re-exports them alongside the shared primitives from `./shared` so
 * callers can pull the whole reducer vocabulary from a single import site.
 */

export { applyPlayCard } from './play'
export { applyDiscard } from './discard'
export { applyGo } from './go'
export { applyReadyNextHand } from './readyNextHand'
export * from './shared'
