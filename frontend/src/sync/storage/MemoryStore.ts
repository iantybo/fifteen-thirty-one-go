/**
 * In-memory {@link KeyValueStore}.
 *
 * Used by tests and in any environment where a durable browser store is
 * unavailable (SSR, no-DOM runtimes). The engine falls back to this so it stays
 * "optimistic but not durable" rather than crashing.
 */

import type { KeyValueStore } from './KeyValueStore'

export class MemoryStore implements KeyValueStore {
  private map = new Map<string, string>()

  getItem(key: string): string | null {
    return this.map.has(key) ? (this.map.get(key) as string) : null
  }

  setItem(key: string, value: string): void {
    this.map.set(key, value)
  }

  removeItem(key: string): void {
    this.map.delete(key)
  }

  /** Test helper: number of keys currently held. */
  size(): number {
    return this.map.size
  }
}
