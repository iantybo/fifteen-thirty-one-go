# Action Lifecycle

> Part of the [Optimistic Sync Engine docs](./README.md). See also [reconciliation.md](./reconciliation.md), [offline-and-durability.md](./offline-and-durability.md).

Every dispatched action is tracked as a `QueuedAction` (see
`frontend/src/sync/types.ts`) and moves through a small state machine as it is
applied, delivered, and reconciled. This document describes that machine, the
transient-vs-permanent branching, and the reconnect requeue path.

## The state machine

```
                     dispatch()
                         │
                         ▼
   ┌───────────────▶  pending  ──────────────────────────────────┐
   │                     │  flush()                                │
   │  transient failure  ▼                                         │
   │  (recordAttempt +  inflight                                   │
   │   backoff, revert   │                                         │
   │   to pending)       ├── success ──▶ confirmed ──reconcile──▶ (dropped)
   │                     │                                         │
   └─────────────────────┤                                         │
                         │                                         │
       reconnect         └── 4xx (perm) ─▶ rejected ─▶ (dropped +  │
    (requeueInflight)                       rolled back + surfaced)│
                                                                   │
        reducer can no longer apply on new base ──────────────────┘
                          (skipped during fold ⇒ dropped)
```

`QueuedActionStatus` is one of `pending | inflight | confirmed | rejected`.

## States

| Status | Meaning | Rendered in `optimisticState`? |
| --- | --- | --- |
| `pending` | Enqueued, not yet sent (or awaiting backoff/reconnect). | Yes — folded in. |
| `inflight` | A delivery attempt is in progress. | Yes — folded in. |
| `confirmed` | The server accepted the delivery; awaiting the reconcile that drops it. | No — its effect comes from the snapshot. |
| `rejected` | The server refused it (permanent); being rolled back. | No — dropped from queue. |

Only `pending` and `inflight` actions are folded into optimistic state
(`recomputeOptimistic` filters on exactly those two). A `confirmed` action's
effect arrives via the authoritative snapshot instead, avoiding double-counting.

## Transitions in `flush()`

`flush()` (in `engine.ts`) processes outstanding actions **oldest first, one at
a time**, because cribbage moves are order-dependent.

```
setStatus(inflight)
   │
   ├─ gameApi.moveGame / nextHand succeeds
   │       └─▶ setStatus(confirmed); continue to next action
   │
   └─ throws
         ├─ recordAttempt(clientId)   (increments attempts)
         │
         ├─ isPermanentError(e)  →  PERMANENT
         │       setStatus(rejected, msg); queue.remove; recomputeOptimistic;
         │       emit; continue (a later action may be independent)
         │
         └─ otherwise             →  TRANSIENT
                 setStatus(pending, msg); emit;
                 scheduleFlush(backoffDelayMs(attempts)); return (stop this pass)
```

### Transient vs. permanent

Classification lives in `errors.ts` (`isPermanentError`):

| Class | HTTP / cause | Handling |
| --- | --- | --- |
| **Transient** | Network error, 5xx, 408 (timeout), 429 (rate limit) | Revert to `pending`, back off, retry. The whole flush pass stops so ordering is preserved. |
| **Permanent** | 4xx except 408/429 (illegal move, out of turn, duplicate) | Mark `rejected`, remove from queue, re-fold optimistic state (rollback), surface to the user. Processing continues with later actions. |

### Backoff

Transient retries use `backoffDelayMs(attempts)` from `backoff.ts`, an
exponential schedule: `0, 250ms, 500ms, 1s, …` capped at **15s**. Because a
transient failure stops the flush pass, only the head-of-line action is retried
until it succeeds, preserving order.

## Rollback

Rollback happens two ways, both of which remove the action from the durable
queue and re-derive optimistic state so the board reverts to authoritative truth:

1. **Eager rollback during flush** — a permanent (4xx) delivery failure. The
   action never lands; `recomputeOptimistic()` drops its predicted effect
   immediately.
2. **Reconcile-time rollback** — an action the reducer can no longer apply to a
   newer authoritative base is `skipped` during the fold and removed. See
   [reconciliation.md](./reconciliation.md).

## Reconnect requeue

When the WebSocket reopens (`ws_open`), the status of any `inflight` action is
unknown — the request may or may not have reached the server. Rather than guess,
the engine reverts every `inflight` action back to `pending`:

```ts
this.ws.on('ws_open', () => {
  this.connected = true
  this.queue.requeueInflight()   // inflight → pending
  this.emit()
  void this.resync()             // pull authoritative snapshot + reconcile
})
```

Re-offering is safe because client ids are stable across retries
(`c:<gameId>:<userId>:<seq>`), so a server that dedupes on client id will not
double-apply. Even a server that does not yet dedupe is protected by
reconciliation: a superseded action is dropped as `accepted`/`skipped`. See
[api-contract.md](./api-contract.md) for the resync semantics and
[glossary.md](./glossary.md) for the client-id format.
