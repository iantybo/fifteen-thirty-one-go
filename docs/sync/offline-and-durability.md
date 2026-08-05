# Offline & Durability

> Part of the [Optimistic Sync Engine docs](./README.md). See also [action-lifecycle.md](./action-lifecycle.md), [architecture.md](./architecture.md).

A core promise of the engine is that a dropped socket mid-hand does not lose
in-flight moves. That promise rests on the durable action queue in
`frontend/src/sync/queue.ts` (`ActionQueue`), which persists its state to
`localStorage` and rehydrates it on load. This document covers persistence
keying, corruption handling, and the fallback to an in-memory store.

## What durability buys you

| Scenario | Without durability | With the durable queue |
| --- | --- | --- |
| Tab reload mid-hand | Pending moves lost. | Rehydrated from storage; flushed on `start()`. |
| Socket drop | In-flight moves lost. | Kept `pending`/`inflight`; requeued on reconnect. |
| Browser crash | Everything lost. | Queue survives; replayed next session. |

## The storage seam: `KeyValueStore`

The queue does not talk to `localStorage` directly. It depends on a small
`KeyValueStore` interface, injected via `EngineDeps.store`:

```
interface KeyValueStore {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}
```

This is the seam that makes durability both testable and degradable:

- In production the default store wraps `window.localStorage`.
- In tests a plain in-memory `Map`-backed store is injected for determinism.
- When `localStorage` is unavailable or throws, the queue falls back to the
  in-memory store automatically (see [graceful degradation](#graceful-degradation-to-memorystore)).

## Per-game keys

The queue persists **one key per game** so that concurrent or sequential games
never clobber each other's queues, and so a single game's queue can be cleared
independently:

```
sync:queue:<gameId>        e.g. sync:queue:42
```

Each key holds a JSON-serialized array of `QueuedAction` records in author
order. On every mutation (`enqueue`, `setStatus`, `recordAttempt`, `remove`,
`requeueInflight`) the queue re-serializes and writes the whole array back —
small (a queue is a handful of moves) and simple to reason about. On
construction the queue reads its key and rehydrates.

```
 construct ActionQueue(gameId, store)
       │
       ▼
 store.getItem("sync:queue:<gameId>")
       │
       ├─ null          ─▶ start empty
       ├─ valid JSON    ─▶ rehydrate QueuedAction[]  (preserves seq order)
       └─ corrupt/invalid ─▶ discard + start empty  (see below)
```

## Corrupt payload handling

Persisted data can be truncated, hand-edited, or written by an incompatible
older version. The queue treats its persisted payload as **untrusted**:

- `JSON.parse` is wrapped; a parse error is swallowed and the queue starts empty
  rather than throwing during construction.
- A parsed value that is not an array, or whose entries do not have the expected
  shape (`clientId`, `gameId`, `action`, `status`, `seq`), is rejected as
  corrupt and discarded.
- Discarding is safe by design: a lost queue means at worst a few un-replayed
  optimistic moves, and the next reconcile pulls authoritative truth anyway. The
  engine prefers **losing an unverifiable optimistic prediction** over crashing
  the game screen.

This behavior is exercised directly by `queue.test.ts` (see
[testing-strategy.md](./testing-strategy.md)).

## Graceful degradation to MemoryStore

`localStorage` is not always available or reliable:

| Condition | Symptom |
| --- | --- |
| Server-side rendering / no `window` | `localStorage` undefined. |
| Private / incognito modes (some browsers) | Access throws or quota is 0. |
| Storage quota exceeded | `setItem` throws `QuotaExceededError`. |
| Storage disabled by policy | Access throws `SecurityError`. |

In every case the queue **degrades rather than crashes**: it catches the failure
and falls back to an in-memory store for the remainder of the session.

```
 try localStorage.setItem(probe) / getItem
      │
      ├─ ok        ─▶ use localStorage-backed store  (durable)
      └─ throws    ─▶ use MemoryStore                 (optimistic, not durable)
```

The consequence of degradation is precise and bounded: the engine stays fully
**optimistic** (the board still moves instantly and the flush/retry/reconcile
machinery all work) but is no longer **durable** across reloads. A `QuotaExceeded`
error on a later `setItem` (after a healthy start) is likewise swallowed — the
in-memory copy remains authoritative for the session and the queue keeps
functioning.

This mirrors the design principle stated in the top-level doc: *"the engine
stays optimistic but not durable, rather than crashing."* See
[faq.md](./faq.md#what-happens-when-localstorage-is-full-or-unavailable) for the
user-visible impact.
