/**
 * Optimistic Sync Engine — public barrel.
 *
 * Import surface for the rest of the app. See `types.ts` for the module-level
 * overview of why the engine exists and how optimistic sync replaces the old
 * request→await→refetch model.
 */

export { SyncEngine, toWireMove, isPermanentError, type EngineDeps } from './engine'
export { useSyncEngine, type UseSyncEngineResult } from './useSyncEngine'
export { applyAction, foldActions, cloneState, cardToCode, cardValue15, type ApplyResult } from './reducer'
export {
  ActionQueue,
  MemoryStore,
  backoffDelayMs,
  defaultStore,
  SYNC_QUEUE_PREFIX,
  type KeyValueStore,
} from './queue'
export type {
  EngineGameState,
  EngineListener,
  QueuedAction,
  QueuedActionStatus,
  ReconcileResult,
  ResyncResponse,
  Revision,
  SyncAction,
  Unsubscribe,
} from './types'
