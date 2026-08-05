# Testing Strategy

> Part of the [Optimistic Sync Engine docs](./README.md). See also [architecture.md](./architecture.md), [action-lifecycle.md](./action-lifecycle.md).

The sync engine is designed to be testable without a real clock, a real
network, real storage, or a running backend. This is not an accident — the
purity of the reducer and the dependency-injection seams on the engine exist
specifically so behavior can be asserted deterministically.

## Test suites

| Suite | Location | Covers |
| --- | --- | --- |
| Reducer | `frontend/src/sync/reducer.test.ts` | Each action kind, rejections, immutability of inputs, folding order, `skipped` output. |
| Queue | `frontend/src/sync/queue.test.ts` | Ordering, idempotency, persistence/rehydration, corrupt-payload handling, backoff bookkeeping, `requeueInflight`. |
| Engine | `frontend/src/sync/engine.test.ts` | Optimistic apply, flush, permanent-rejection rollback, `game_update` reconciliation, stale-snapshot ignore, connection tracking, reconnect requeue. |
| Resync | `backend/internal/handlers/resync_test.go` | The pure `reconcilePending` acceptance policy against varied `last_revision`/`serverRevision`/pending combinations. |

Run them:

```bash
# frontend
cd frontend && npm install && npm test
# backend
cd backend && go test ./internal/handlers/
```

## The layered testing pyramid

```
        ┌────────────────────────────────────────┐
        │ engine.test.ts (integration of parts)   │  ← fake clock, fake ws,
        │  dispatch → flush → reconcile           │    fake api, memory store
        ├────────────────────────────────────────┤
        │ queue.test.ts        reducer.test.ts    │  ← pure units, no I/O
        ├────────────────────────────────────────┤
        │ resync_test.go (pure reconcilePending)  │  ← pure fn, no DB
        └────────────────────────────────────────┘
```

The pure layers (reducer, queue logic, `reconcilePending`) carry the bulk of the
assertions because they are cheap and total. The engine tests wire the real
parts together but stub the *edges* via `EngineDeps`.

## Dependency-injection seams (`EngineDeps`)

`SyncEngine`'s constructor takes `Partial<EngineDeps>`, and every
non-deterministic dependency is one of these seams:

| Dep | Production default | In tests |
| --- | --- | --- |
| `now` | `() => Date.now()` | A controllable counter, so timestamps and `createdAt` are fixed. |
| `setTimer` | `setTimeout`/`clearTimeout` wrapper | A manual scheduler: capture the callback, advance "time" by invoking it, assert flush timing/backoff without real waiting. |
| `store` | `localStorage`-backed store | An in-memory `Map`-backed `KeyValueStore` for deterministic persistence + corruption tests. |
| `makeWs` | `() => new WsClient()` | A fake `WsClient` whose `on`/emit can be driven to simulate `ws_open`, `ws_close`, `game_update`. |
| `gameApi` | the real `api` | A stub implementing `getGame` / `moveGame` / `nextHand` that resolves, rejects with a transient error, or rejects with a permanent 4xx on command. |

A typical engine test constructs the engine with all five stubs, `start()`s it,
`dispatch()`es actions, drives the fake timer and fake ws, and asserts against
`getState()` / `lastReconcile`.

```ts
const now = makeCounterClock()
const timers = makeManualScheduler()
const api = makeStubApi()
const ws = new FakeWsClient()
const engine = new SyncEngine(gameId, userId, {
  now, setTimer: timers.setTimer, store: new MemoryStore(),
  makeWs: () => ws, gameApi: api,
})
```

## Determinism requirements

The whole approach depends on removing every source of nondeterminism from the
code under test:

1. **No wall-clock reads.** All time flows through `now` / `setTimer`. The
   reducer reads no clock at all.
2. **No randomness.** Client ids come from `IdGenerator` (`c:<gameId>:<userId>:<seq>`)
   with a monotonic `seq` — no `Math.random()`. Ids are reproducible across runs
   and stable across retries.
3. **Pure, total reducer.** `applyAction` / `foldActions` never mutate inputs
   (`cloneState` first) and never throw — an un-appliable action returns a
   rejection / lands in `skipped`. This lets tests re-fold the same queue on many
   bases and get identical results, and lets the engine rebase safely.
4. **Pure acceptance policy.** `reconcilePending` is a pure function of
   `(request, serverRevision)` — no DB, no time — so `resync_test.go` enumerates
   cases as plain table-driven tests.
5. **Ordered, one-at-a-time flush.** Because `flush()` processes oldest-first and
   stops the pass on a transient failure, tests can assert exact delivery order
   and backoff scheduling.

## What each suite is really pinning down

- **reducer.test.ts** — that optimistic prediction is correct and side-effect
  free (no score prediction, correct turn/pegging/hand updates), and that
  folding is order-sensitive and produces a `skipped` set.
- **queue.test.ts** — that durability round-trips, that corrupt payloads never
  crash construction, and that backoff attempt counting is monotonic. See
  [offline-and-durability.md](./offline-and-durability.md).
- **engine.test.ts** — the [action lifecycle](./action-lifecycle.md) and
  [reconciliation](./reconciliation.md) end to end: rollback on 4xx, retry on
  5xx, ignore of stale snapshots, requeue on reconnect.
- **resync_test.go** — the [server acceptance policy](./api-contract.md#acceptance-policy).
