/**
 * Durable action queue for the sync engine.
 *
 * The queue is the "memory" of optimistic sync. Every action the player takes
 * is appended here *before* it is sent, so that:
 *
 *  - a disconnect or page reload does not lose in-flight moves (the queue is
 *    persisted to `localStorage` and rehydrated on construction), and
 *  - the engine can re-fold and re-send actions in their original author order
 *    after a reconnect.
 *
 * The queue is intentionally dumb: it stores, orders, and mutates status. All
 * policy (when to flush, how to reconcile) lives in `engine.ts`. This split
 * keeps the queue trivially unit-testable with an in-memory storage double.
 *
 * ## Persistence format
 *
 * One storage key per game (`SYNC_QUEUE_PREFIX + gameId`) holding a JSON array
 * of `QueuedAction`. Per-game keys mean two open games never clobber each
 * other, and a finished game's queue can be dropped independently.
 */

import type { QueuedAction, QueuedActionStatus, SyncAction, Revision } from './types'
import { backoffDelayMs } from './backoff'
import { deserializeQueue, serializeQueue } from './serialization'
import { MemoryStore, resolveStore, SYNC_QUEUE_PREFIX, type KeyValueStore } from './storage'

// Backward-compatible re-exports. These primitives moved to focused modules
// (`backoff.ts`, `serialization.ts`, `storage/`) during the split, but historical
// importers pull them from `./queue`, so we keep the names available here.
export { backoffDelayMs }
export { MemoryStore, SYNC_QUEUE_PREFIX, type KeyValueStore }
export type { QueuedAction, QueuedActionStatus }

/** Resolve the default storage backend (browser localStorage or in-memory). */
export function defaultStore(): KeyValueStore {
  return resolveStore()
}

function storageKey(gameId: number): string {
  return `${SYNC_QUEUE_PREFIX}${gameId}`
}

/**
 * True when a raw payload is a legitimately-empty array (`"[]"` with optional
 * whitespace). Used to distinguish "persisted, but empty" (keep the key) from
 * "corrupt/incompatible, produced nothing" (drop the key).
 */
function isEmptyArrayPayload(raw: string): boolean {
  return raw.trim() === '[]'
}

/**
 * The durable queue for a single game.
 *
 * Ordering invariant: `list()` always returns actions sorted by `seq` ascending,
 * which is the order they were authored. The engine relies on this to fold and
 * flush deterministically.
 */
export class ActionQueue {
  private items: QueuedAction[] = []
  private nextSeq = 0

  constructor(
    private readonly gameId: number,
    private readonly store: KeyValueStore = defaultStore(),
  ) {
    this.rehydrate()
  }

  /** Load and validate any persisted actions for this game. */
  private rehydrate(): void {
    const raw = this.store.getItem(storageKey(this.gameId))
    // Validation + parse error handling lives in serialization.ts; it returns
    // [] for any malformed payload and drops individual bad entries.
    const items = deserializeQueue(raw)
    if (raw && items.length === 0 && !isEmptyArrayPayload(raw)) {
      // The payload existed but produced nothing usable (corrupt / incompatible):
      // clear it so we start from a clean slate.
      this.store.removeItem(storageKey(this.gameId))
    }
    this.items = items
    // Restore the sequence counter so newly-enqueued actions sort after
    // rehydrated ones.
    this.nextSeq = this.items.reduce((max, a) => Math.max(max, a.seq + 1), 0)
    this.sort()
  }

  private sort(): void {
    this.items.sort((a, b) => a.seq - b.seq)
  }

  private persist(): void {
    try {
      this.store.setItem(storageKey(this.gameId), serializeQueue(this.items))
    } catch {
      // Quota/serialization failure: keep operating in-memory. Losing durability
      // is acceptable; crashing the game is not.
    }
  }

  /** All actions in author order (defensive copy). */
  list(): QueuedAction[] {
    return this.items.map((a) => ({ ...a, action: { ...a.action } as SyncAction }))
  }

  /** Actions still needing delivery (pending or inflight), in author order. */
  outstanding(): QueuedAction[] {
    return this.list().filter((a) => a.status === 'pending' || a.status === 'inflight')
  }

  /** Look up an action by its stable client id. */
  get(clientId: string): QueuedAction | undefined {
    const found = this.items.find((a) => a.clientId === clientId)
    return found ? { ...found, action: { ...found.action } as SyncAction } : undefined
  }

  /**
   * Append a new action. `clientId` must be unique; enqueuing a duplicate id is
   * a no-op that returns the existing entry (idempotency guard — protects
   * against double-submits from rapid clicks or retried callbacks).
   */
  enqueue(clientId: string, action: SyncAction, baseRevision: Revision, createdAt: number): QueuedAction {
    const existing = this.items.find((a) => a.clientId === clientId)
    if (existing) return { ...existing, action: { ...existing.action } as SyncAction }
    const item: QueuedAction = {
      clientId,
      gameId: this.gameId,
      action,
      status: 'pending',
      baseRevision,
      seq: this.nextSeq++,
      createdAt,
      attempts: 0,
    }
    this.items.push(item)
    this.sort()
    this.persist()
    return { ...item, action: { ...item.action } as SyncAction }
  }

  /** Update the status of an action in place. No-op if the id is unknown. */
  setStatus(clientId: string, status: QueuedActionStatus, lastError?: string): void {
    const item = this.items.find((a) => a.clientId === clientId)
    if (!item) return
    item.status = status
    if (lastError !== undefined) item.lastError = lastError
    this.persist()
  }

  /** Increment the delivery-attempt counter for backoff bookkeeping. */
  recordAttempt(clientId: string): void {
    const item = this.items.find((a) => a.clientId === clientId)
    if (!item) return
    item.attempts += 1
    this.persist()
  }

  /** Remove a set of actions (e.g. confirmed by the server). */
  remove(clientIds: Iterable<string>): void {
    const drop = new Set(clientIds)
    if (drop.size === 0) return
    this.items = this.items.filter((a) => !drop.has(a.clientId))
    this.persist()
  }

  /**
   * Reset every inflight action back to pending. Called on reconnect: anything
   * that was mid-flight when the socket dropped has an unknown fate, so we
   * re-offer it to the server, which dedupes by `clientId`.
   */
  requeueInflight(): void {
    let changed = false
    for (const item of this.items) {
      if (item.status === 'inflight') {
        item.status = 'pending'
        changed = true
      }
    }
    if (changed) this.persist()
  }

  /** Number of actions currently tracked. */
  size(): number {
    return this.items.length
  }

  /** Drop the entire queue and its persisted key (e.g. game finished/quit). */
  clear(): void {
    this.items = []
    this.nextSeq = 0
    this.store.removeItem(storageKey(this.gameId))
  }
}

