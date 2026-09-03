/**
 * The storage contract used by the durable action queue, plus the resolver that
 * picks a concrete backend at runtime.
 *
 * Kept deliberately narrow (three methods) so the engine never accidentally
 * couples to the full Web Storage API. Concrete implementations live alongside
 * this file: `MemoryStore` (in-memory, for tests/SSR) and `LocalStorageStore`
 * (browser-backed, durable).
 */

/** Minimal key/value contract. `window.localStorage` satisfies it directly. */
export interface KeyValueStore {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}
