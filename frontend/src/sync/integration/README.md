# Sync engine — integration tests

These suites exercise a **real `SyncEngine`** end-to-end against test doubles for
its three side-effecting seams: the WebSocket, the HTTP transport, and the clock.
Where the unit tests (`reducer.test.ts`, `queue.test.ts`, `reconciler.test.ts`)
verify each part in isolation, these tests verify that the parts are wired
together correctly — that a UI dispatch actually travels
`optimistic apply → durable enqueue → ordered flush → reconcile`.

## What each suite covers

| Suite | Covers |
| --- | --- |
| `dispatch-flush.test.ts` | A dispatch applies optimistically and is delivered; multiple dispatches flush in author order; the queue empties after the post-flush reconcile; `pending`/`inflight` counts track outstanding work; `ready_next_hand` routes to `nextHand` rather than `moveGame`. |
| `reconnect.test.ts` | `ws_close`/`ws_open` toggle connection state; an action that was `inflight` when the socket dropped is requeued to `pending` and re-sent after reconnect; a `game_update` triggers exactly one `getGame` refetch and the board follows the newer server state. |
| `offline-replay.test.ts` | A transient (non-4xx) transport failure leaves the action `pending` with an incremented attempt count and a **backoff** timer parked; retries grow per `backoffDelayMs`; once the network recovers the queued actions drain in author order. |

## Why the DI seams make these deterministic

The engine takes an `EngineDeps` bag (`now`, `setTimer`, `store`, `makeWs`,
`gameApi`) precisely so tests never touch real time, sockets, storage, or a
backend. Each suite builds deps with a small `makeDeps` helper (mirrored from
`engine.test.ts`) that swaps in:

- **`now`** — frozen to a constant, so `createdAt` and backoff math are
  reproducible.
- **`setTimer`** — zero-delay flushes run on a microtask, while *delayed*
  (backoff) timers are parked in an array the test fires by hand. No real clock,
  no `vi.useFakeTimers`; a retry loop becomes a value you advance explicitly.
- **`store`** — an in-memory `MemoryStore`, so the durable queue is isolated
  per test.
- **`makeWs`** — [`FakeWs`](../__fixtures__/ws.ts): the test drives `ws_open`,
  `ws_close`, and `game_update` via `fire(...)`.
- **`gameApi`** — [`FakeTransport`](../__fixtures__/transport.ts): records every
  `getGame`/`moveGame`/`nextHand` call in order and can be told to throw, so
  assertions read against recorded argument arrays instead of mock matchers.

Because every source of nondeterminism is an injected value, the same dispatch
produces the same recorded calls on every run.

See the project-level [testing strategy](../../../../docs/sync/testing-strategy.md)
for how these integration suites sit atop the pure-unit layer.
