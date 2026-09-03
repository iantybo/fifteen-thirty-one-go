/**
 * Optimistic Sync Engine — public barrel.
 *
 * Import surface for the rest of the app. See `types.ts` for the module-level
 * overview of why the engine exists and how optimistic sync replaces the old
 * request→await→refetch model, and `docs/sync/` for the full design docs.
 */

// Orchestrator + React binding
export { SyncEngine, toWireMove, type EngineDeps } from './engine'
export { useSyncEngine, type UseSyncEngineResult } from './useSyncEngine'

// Reducer (facade over reducers/*)
export { applyAction, foldActions, cloneState, cardToCode, cardValue15, type ApplyResult } from './reducer'

// Durable queue + storage
export { ActionQueue, backoffDelayMs, defaultStore } from './queue'
export { MemoryStore, LocalStorageStore, resolveStore, SYNC_QUEUE_PREFIX, type KeyValueStore } from './storage'

// Pure engine sub-modules
export { planReconcile, dropReconciled, type ReconcileInput } from './reconciler'
export { decideOnError, orderForDelivery, type FlushDecision } from './flusher'
export { isPermanentError, isTransientError, errorMessage, TRANSIENT_4XX } from './errors'
export { IdGenerator, parseClientId, isClientId, type ParsedClientId } from './clientId'
export { INITIAL_REVISION, isNewer, isStale, bump, max } from './revision'
export { cumulativeBackoffMs, BACKOFF_BASE_MS, BACKOFF_CAP_MS } from './backoff'
export { isSyncAction, isQueuedAction, serializeQueue, deserializeQueue } from './serialization'

// Transport abstraction
export { RealTransport, type GameTransport } from './transport'

// Telemetry
export {
  noopTelemetry,
  RecordingTelemetry,
  ConsoleTelemetry,
  type Telemetry,
  type TelemetryEvent,
} from './telemetry'

// Types
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
