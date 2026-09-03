# Architecture

> Part of the [Optimistic Sync Engine docs](./README.md). See also [reconciliation.md](./reconciliation.md), [action-lifecycle.md](./action-lifecycle.md).

This document maps the components of the sync engine and traces a game action
end to end. The engine is a **per-game** controller: one `SyncEngine` instance
exists per open game and is torn down when the game screen unmounts.

## Components

The frontend engine lives entirely under `frontend/src/sync/` and is
deliberately split so that the pure logic (reducer, queue) can be tested without
React, a real clock, or a real network. The backend contributes a single
reconciliation endpoint.

```
   React layer                Engine core                  Boundaries
  ┌──────────────┐        ┌────────────────────┐        ┌───────────────────┐
  │ GamePage     │        │ SyncEngine         │        │ api/client        │
  │ useSyncEngine│──────▶ │  ├─ reducer.ts     │───────▶│  getGame          │
  │ (React hook) │◀────── │  ├─ queue.ts       │        │  moveGame         │
  └──────────────┘ state  │  ├─ backoff.ts     │        │  nextHand         │
                          │  ├─ errors.ts      │        ├───────────────────┤
                          │  └─ clientId.ts    │◀───────│ ws/wsClient       │
                          └────────────────────┘ events │  game_update      │
                                    │                    └───────────────────┘
                                    │ POST /api/games/:id/resync
                                    ▼
                          ┌────────────────────┐
                          │ resync.go (backend)│
                          │  ResyncHandler     │
                          │  reconcilePending  │
                          └────────────────────┘
```

## Data flow: one dispatched action

```
 1. GamePage → useSyncEngine.dispatch(action)
        │
 2. SyncEngine.dispatch(action)
        ├─ ids.next() ─────────────▶ clientId = c:<gameId>:<userId>:<seq>
        ├─ queue.enqueue(clientId, action, baseRevision, now())   (durable)
        ├─ recomputeOptimistic() ─▶ optimisticState = fold(confirmed, pending)
        ├─ emit() ─────────────────▶ React re-renders  (board moves NOW)
        └─ scheduleFlush(0)
        │
 3. SyncEngine.flush()  (oldest first, one at a time)
        ├─ setStatus(inflight)
        ├─ gameApi.moveGame / gameApi.nextHand
        ├─ success  ─▶ setStatus(confirmed)
        ├─ 4xx      ─▶ setStatus(rejected) + remove + recomputeOptimistic
        └─ transient▶ setStatus(pending) + recordAttempt + scheduleFlush(backoff)
        │
 4. server broadcasts game_update over WS (or flush() calls refetchAndReconcile)
        │
 5. SyncEngine.reconcile(snapshot, revision, accepted, rejected)
        └─ optimisticState = fold(newSnapshot.state, stillPending)
```

Steps 1–2 are synchronous: the UI never waits on the network to render the
predicted outcome. Steps 3–5 are asynchronous and drive the board back toward
authoritative truth. See [action-lifecycle.md](./action-lifecycle.md) for the
per-action state machine and [reconciliation.md](./reconciliation.md) for what
happens in step 5.

## Module responsibilities

### Frontend (`frontend/src/sync/*`)

| Module | Responsibility | Key exports |
| --- | --- | --- |
| `types.ts` | Shared, framework-agnostic types + module design narrative. | `SyncAction`, `QueuedAction`, `EngineGameState`, `ReconcileResult`, `ResyncResponse` |
| `reducer.ts` | Pure, total, deterministic optimistic reducer. Predicts turn order, pegging total, and which of *my* cards left my hand — never scores. | `applyAction`, `foldActions`, `cloneState` |
| `queue.ts` | Durable, ordered, persisted action queue with backoff bookkeeping and graceful degradation. | `ActionQueue`, `KeyValueStore` |
| `backoff.ts` | Pure exponential backoff schedule (0, 250ms, 500ms, 1s, … capped at 15s). | `backoffDelayMs` |
| `errors.ts` | Classifies caught errors as transient vs. permanent; extracts messages. | `isPermanentError`, `errorMessage` |
| `clientId.ts` | Deterministic, monotonic client-id generation (`c:<gameId>:<userId>:<seq>`). No `Math.random()`. | `IdGenerator` |
| `engine.ts` | Orchestrator: dispatch → optimistic → enqueue → flush → reconcile; owns WS wiring, timers, and `EngineDeps` injection. | `SyncEngine`, `EngineDeps`, `toWireMove` |
| `useSyncEngine.ts` | React binding hook: constructs, starts, subscribes to, and stops a `SyncEngine`. | `useSyncEngine` |
| `index.ts` | Public barrel re-exporting the stable surface. | — |

### Backend (`backend/internal/handlers/resync.go`)

| Symbol | Responsibility |
| --- | --- |
| `ResyncHandler(db)` | Gin handler for `POST /api/games/:id/resync`. Validates the id, authorizes the user as a participant, computes the current revision, builds the user-specific snapshot, and returns `resyncResponse`. |
| `reconcilePending(req, serverRevision)` | Pure acceptance policy: compares the client's `last_revision` to the server revision to decide `accepted`/`rejected` client ids. Unit-testable without a database. |
| `gameRevision(db, gameID)` | Computes the authoritative revision as the count of durably-recorded moves. |

## Injection seams

The engine takes an `EngineDeps` bag so tests can control everything
non-deterministic:

| Dep | Default | Purpose |
| --- | --- | --- |
| `now` | `() => Date.now()` | Timestamps + backoff base. |
| `setTimer` | `setTimeout`/`clearTimeout` wrapper | Flush scheduling. |
| `store` | `localStorage`-backed store | Queue persistence (falls back to memory). |
| `makeWs` | `() => new WsClient()` | WebSocket transport factory. |
| `gameApi` | the real `api` | HTTP surface (`getGame`, `moveGame`, `nextHand`). |

See [testing-strategy.md](./testing-strategy.md) for how these seams are used to
keep the engine deterministic under test.
