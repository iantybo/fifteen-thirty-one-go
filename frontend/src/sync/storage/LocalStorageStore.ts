/**
 * Browser-backed {@link KeyValueStore} plus the runtime resolver.
 *
 * `LocalStorageStore` wraps `window.localStorage` behind the narrow contract.
 * `resolveStore()` probes for a usable `localStorage` (some environments expose
 * the object but throw on write, e.g. Safari private mode) and falls back to an
 * in-memory store when it is unavailable.
 */

import type { KeyValueStore } from './KeyValueStore'
import { MemoryStore } from './MemoryStore'

/** Prefix used to namespace all sync-engine keys. */
export const SYNC_QUEUE_PREFIX = 'fifteen-thirty-one:sync-queue:'

/** A thin wrapper over `window.localStorage` satisfying `KeyValueStore`. */
export class LocalStorageStore implements KeyValueStore {
  constructor(private readonly backing: Storage) {}

  getItem(key: string): string | null {
    return this.backing.getItem(key)
  }

  setItem(key: string, value: string): void {
    this.backing.setItem(key, value)
  }

  removeItem(key: string): void {
    this.backing.removeItem(key)
  }
}

/**
 * Resolve a storage backend. Returns a `LocalStorageStore` when `localStorage`
 * is present AND writable; otherwise an ephemeral `MemoryStore`. Never throws.
 */
export function resolveStore(): KeyValueStore {
  try {
    if (typeof localStorage !== 'undefined') {
      const probe = `${SYNC_QUEUE_PREFIX}__probe__`
      localStorage.setItem(probe, '1')
      localStorage.removeItem(probe)
      return new LocalStorageStore(localStorage)
    }
  } catch {
    // Present but unwritable (quota, private mode): fall through to memory.
  }
  return new MemoryStore()
}
