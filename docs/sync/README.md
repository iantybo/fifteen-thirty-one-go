# Optimistic Sync Engine — Documentation Index

> Status: implemented · Owner: frontend/game · Related: `frontend/src/sync/`, `backend/internal/handlers/resync.go`

This directory documents the **Optimistic Sync Engine** in depth. It is the
companion to the top-level design doc, [`../optimistic-sync-engine.md`](../optimistic-sync-engine.md),
which is the best starting point if you have never seen this feature before.

The engine changes the client's core data flow from **request → await → refetch**
to **optimistic application + reconciliation**: game actions are applied locally
by a pure reducer, queued durably in `localStorage`, flushed to the server with
retry/backoff, and reconciled against authoritative snapshots delivered over
HTTP or via WebSocket `game_update` events.

```
 dispatch(action) ─▶ reducer (optimistic) ─▶ queue (durable) ─▶ flush ─▶ server
        ▲                                                                   │
        └────────────── reconcile(snapshot) ◀── game_update / resync ◀──────┘
```

## Documents

| Doc | Summary |
| --- | --- |
| [architecture.md](./architecture.md) | The components, the end-to-end data flow diagram, and a responsibilities table covering every module under `frontend/src/sync/*` and the backend `resync.go`. Read this to build a mental model of how the pieces fit together. |
| [reconciliation.md](./reconciliation.md) | The reconcile algorithm in detail: stale-snapshot rejection via revisions, confirm/rebase/rollback decisions, and the fold-onto-new-base step that recomputes optimistic state on top of each fresh authoritative snapshot. |
| [action-lifecycle.md](./action-lifecycle.md) | The `pending → inflight → confirmed → rejected` state machine, the distinction between transient retry and permanent rollback, and how inflight actions are requeued on reconnect. |
| [offline-and-durability.md](./offline-and-durability.md) | How the queue persists to `localStorage` with one key per game, how corrupt payloads are handled, and how the engine degrades gracefully to an in-memory store when storage is unavailable. |
| [api-contract.md](./api-contract.md) | The `POST /api/games/:id/resync` request/response shapes (`ResyncResponse`), status codes, revision semantics, and worked example JSON for each case. |
| [testing-strategy.md](./testing-strategy.md) | How the reducer, queue, engine, and resync handler are tested, the dependency-injection seams (`EngineDeps`: `now`, `setTimer`, `store`, `makeWs`, `gameApi`), and the determinism requirements that make it all possible. |
| [glossary.md](./glossary.md) | Definitions of the vocabulary used throughout: revision, client id, optimistic state, confirmed snapshot, reconcile, rebase, fold, and transient vs. permanent error. |
| [faq.md](./faq.md) | Answers to the questions an engineer typically asks: why scores are not predicted, what happens on a double-submit, what happens when `localStorage` is full, and more. |

## See also

- [`../optimistic-sync-engine.md`](../optimistic-sync-engine.md) — the primary design doc.
- [`../../frontend/src/sync/README.md`](../../frontend/src/sync/README.md) — the developer-facing module README (quick start + module map).
- [`../../frontend/src/sync/types.ts`](../../frontend/src/sync/types.ts) — the source-of-truth type definitions, with inline narrative.

## Reading order

1. [`../optimistic-sync-engine.md`](../optimistic-sync-engine.md) for the motivation and high-level shape.
2. [architecture.md](./architecture.md) for the component map.
3. [action-lifecycle.md](./action-lifecycle.md) and [reconciliation.md](./reconciliation.md) for the two central mechanisms.
4. [api-contract.md](./api-contract.md) and [offline-and-durability.md](./offline-and-durability.md) for the boundaries.
5. [glossary.md](./glossary.md) and [faq.md](./faq.md) as references.
