import { describe, it, expect } from 'vitest'
import { LocalStorageStore, resolveStore, SYNC_QUEUE_PREFIX } from './LocalStorageStore'
import { MemoryStore } from './MemoryStore'
import type { KeyValueStore } from './KeyValueStore'

/** A minimal Storage double sufficient for LocalStorageStore. */
function fakeStorage(): Storage {
  const map = new Map<string, string>()
  return {
    get length() {
      return map.size
    },
    clear: () => map.clear(),
    getItem: (k: string) => (map.has(k) ? (map.get(k) as string) : null),
    key: (i: number) => Array.from(map.keys())[i] ?? null,
    removeItem: (k: string) => void map.delete(k),
    setItem: (k: string, v: string) => void map.set(k, v),
  } as Storage
}

describe('LocalStorageStore', () => {
  it('delegates to the backing Storage', () => {
    const store: KeyValueStore = new LocalStorageStore(fakeStorage())
    store.setItem('k', 'v')
    expect(store.getItem('k')).toBe('v')
    store.removeItem('k')
    expect(store.getItem('k')).toBeNull()
  })
})

describe('resolveStore', () => {
  it('returns a working store in this environment', () => {
    // In the node test env localStorage is undefined, so we expect a MemoryStore.
    const store = resolveStore()
    expect(store).toBeInstanceOf(MemoryStore)
    store.setItem('x', '1')
    expect(store.getItem('x')).toBe('1')
  })
})

describe('SYNC_QUEUE_PREFIX', () => {
  it('is namespaced to the app', () => {
    expect(SYNC_QUEUE_PREFIX.startsWith('fifteen-thirty-one:')).toBe(true)
  })
})
