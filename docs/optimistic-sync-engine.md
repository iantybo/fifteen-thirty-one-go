# Optimistic Sync Engine

> Status: implemented · Owner: frontend/game · Related: `frontend/src/sync/`, `backend/internal/handlers/resync.go`

## Summary

The Optimistic Sync Engine changes how the client talks to the game backend.
Previously every game action followed a **request → await → refetch** pattern:
the UI disabled itself, POSTed the move, waited for the server, then re-fetched
the whole snapshot before the board moved. The engine replaces that with
**optimistic application + reconciliation**:

1. The action is applied locally and immediately by a pure reducer, so the board
   moves on the next frame.
2. The action is appended to a durable, persisted queue that survives reloads
   and disconnects.
3. The queue is flushed to the server with ordering + retry/backoff.
4. The authoritative server snapshot (via HTTP response or WebSocket
   `game_update`) is treated as the source of truth and is **reconciled**
   against the local optimistic state — confirming, rebasing, or rolling back
   optimistic actions.

This is a foundational change to the app's core data flow: the network round
trip is no longer on the critical path for UI responsiveness, and game actions
are no longer lost on a flaky connection.

## Motivation

| Problem (before) | Fix (after) |
| --- | --- |
| Board freezes until the server responds to each move. | Reducer predicts the local outcome instantly. |
| A dropped socket mid-hand loses in-flight moves. | Actions are persisted to `localStorage` and replayed on reconnect. |
| Every action triggers a full `GET /games/:id`. | Snapshots become a reconciliation signal; the queue drives delivery. |

## Architecture

```
                      dispatch(action)
  GamePage ──────────────────────────────────▶ SyncEngine
     ▲                                            │  1. applyAction()  (reducer.ts)
     │ subscribe(state)                           │  2. queue.enqueue() (queue.ts)
     │                                            │  3. scheduleFlush()
     └──────────── EngineGameState ◀──────────────┤
                                                  │  flush(): api.moveGame / api.nextHand
                                                  │
   WsClient ── game_update ──▶ resync() ──▶ reconcile(snapshot, rev, accepted, rejected)
                                                  │
                              api.getGame / POST /games/:id/resync
```

### Modules (`frontend/src/sync/`)

| File | Responsibility |
| --- | --- |
| `types.ts` | Shared types + the module-level design narrative. |
| `reducer.ts` | Pure, total, deterministic optimistic reducer (`applyAction`, `foldActions`). |
| `queue.ts` | Durable, persisted, ordered action queue with backoff (`ActionQueue`). |
| `engine.ts` | Orchestrator: dispatch → optimistic → enqueue → flush → reconcile (`SyncEngine`). |
| `useSyncEngine.ts` | React binding hook. |
| `index.ts` | Public barrel. |

### Backend (`backend/internal/handlers/resync.go`)

`POST /api/games/:id/resync` reconciles a client's outstanding action queue
against authoritative state and returns the current snapshot + accepted/rejected
client ids.

## Key design decisions

### The reducer predicts only what is locally derivable

The client does not know the deck, the opponent's hand, or the server's exact
scoring. The reducer therefore predicts turn order, the pegging total, and which
of *my* cards left my hand — but **never scores**. Predicting an unverifiable
score would guarantee a visible rollback. Peg-board movement on reconciliation
reads naturally as "the server awarded points."

The reducer is **pure, total, and deterministic**: it never mutates input, never
throws (un-appliable actions return `{ rejected }`), and uses no clock or
randomness. This is what makes it unit-testable and lets the engine re-fold the
queue on top of any base snapshot during reconciliation.

### Revisions define ordering, not identity

A game's revision is the count of durable moves. It is monotonically
non-decreasing and cheap to compute. The client only ever *compares* revisions
(newer vs. stale) to reject out-of-order WebSocket deliveries that would regress
the board — it never interprets the absolute value.

### Client ids are stable and deterministic

Each action gets a `c:<gameId>:<userId>:<seq>` id. It is stable across retries so
the server (in a future iteration that persists it) can dedupe, and it is the
identity used for reconciliation and rollback. No `Math.random()` — ids are
reproducible in tests.

### Durability degrades gracefully

The queue persists to `localStorage`, one key per game. If storage is
unavailable (private browsing, SSR, quota errors) the queue falls back to an
in-memory store: the engine stays optimistic but not durable, rather than
crashing.

### Conservative reconciliation policy (server)

The server does not yet persist client action ids, so it cannot match a pending
id to a specific stored move. It uses a safe rule instead:

- **Client behind server revision** → its optimistic actions have been
  superseded by the authoritative snapshot; report them `accepted` (stop
  tracking).
- **Client at server revision** → nothing new landed; leave the actions
  outstanding.

The wire contract already carries the client ids, so a future change to precise
per-action acceptance is non-breaking.

## Action lifecycle

```
 pending ──flush()──▶ inflight ──ack──▶ confirmed ──reconcile──▶ (dropped)
    ▲                    │
    └──── transient ─────┘  (recordAttempt + backoff)
                         │
                         └── 4xx ──▶ rejected ──▶ (dropped + rolled back + surfaced)
```

- **Transient** failures (network, 5xx, 408, 429): stay pending, exponential
  backoff (`backoffDelayMs`: 0, 250ms, 500ms, 1s, … capped at 15s), retried.
- **Permanent** failures (4xx except 408/429): the move was illegal/duplicate;
  drop it, roll back the optimistic prediction, surface to the user.
- On **reconnect**, all `inflight` actions revert to `pending` and are re-offered
  (the server dedupes).

## Testing

| Test | Covers |
| --- | --- |
| `reducer.test.ts` | Each action kind, rejections, immutability, folding order. |
| `queue.test.ts` | Ordering, idempotency, persistence/rehydration, corrupt-payload handling, backoff. |
| `engine.test.ts` | Optimistic apply, flush, permanent-rejection rollback, `game_update` reconciliation, stale-snapshot ignore, connection tracking. |
| `resync_test.go` | The pure `reconcilePending` acceptance policy. |

Run:

```bash
# frontend
cd frontend && npm install && npm test
# backend
cd backend && go test ./internal/handlers/
```

## Limitations & future work

- The server's acceptance policy is coarse (revision comparison). Persisting
  `client_id` on each move would enable precise per-action accept/reject.
- Scores are never predicted; the peg board moves only on reconciliation.
- The engine handles one game per instance; cross-game coordination (e.g. a
  global lobby queue) is out of scope.
