/**
 * Storage adapters for the durable action queue.
 *
 * The queue depends only on the {@link KeyValueStore} interface; these adapters
 * provide concrete backends. See each file for details.
 */

export type { KeyValueStore } from './KeyValueStore'
export { MemoryStore } from './MemoryStore'
export { LocalStorageStore, resolveStore, SYNC_QUEUE_PREFIX } from './LocalStorageStore'
