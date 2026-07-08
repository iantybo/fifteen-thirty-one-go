# FAQ

> Part of the [Optimistic Sync Engine docs](./README.md). See also [glossary.md](./glossary.md), [reconciliation.md](./reconciliation.md).

Common questions from engineers reading or extending the sync engine.

### Why doesn't the reducer predict scores?

The client does not know the deck, the opponent's hand, or the server's exact
scoring logic. A predicted score is *unverifiable*, so it would almost certainly
disagree with the authoritative snapshot and produce a visible rollback — the
peg board jumping backward is jarring. Instead the reducer predicts only what is
locally derivable (turn order, the pegging total, which of *my* cards left my
hand). Peg-board movement then arrives on reconciliation and reads naturally as
"the server awarded points." See [glossary.md](./glossary.md#optimistic-state).

### What happens on a double-submit (the same move sent twice)?

Every action has a **stable client id** (`c:<gameId>:<userId>:<seq>`) that does
not change across retries. A future server that persists client ids can dedupe
directly. Even today, reconciliation protects you: once the server advances,
resync reports the superseded action as `accepted` and the client drops it, so a
re-offer cannot double-apply from the client's point of view. See
[action-lifecycle.md](./action-lifecycle.md#reconnect-requeue).

### What happens when `localStorage` is full or unavailable?

The queue catches the failure and falls back to an **in-memory store** for the
session. The engine stays fully optimistic — the board still moves and
flush/retry/reconcile all work — but the queue is no longer durable across
reloads. It never crashes the game screen. See
[offline-and-durability.md](./offline-and-durability.md#graceful-degradation-to-memorystore).

### What if the persisted queue is corrupt?

On construction the queue treats its persisted payload as untrusted: a JSON
parse error, a non-array value, or entries of the wrong shape are discarded and
the queue starts empty. Losing an unverifiable optimistic prediction is
preferable to crashing; the next reconcile pulls authoritative truth anyway.

### How does the engine avoid the board "jumping backward"?

WebSocket `game_update` deliveries can arrive out of order or be duplicated.
`reconcile()` compares the incoming revision to the current one and **ignores any
snapshot that is not strictly newer** (marking `ignoredStale: true`). Because
the revision is the count of durable moves, this reliably orders deliveries. See
[reconciliation.md](./reconciliation.md#step-0--stale-rejection-via-revisions).

### Transient vs. permanent — how does the engine decide whether to retry?

`isPermanentError` (in `errors.ts`) classifies the caught error. Network
failures, 5xx, 408, and 429 are **transient**: the action stays pending and is
retried with exponential backoff. Any other 4xx is **permanent**: the move was
illegal/duplicate, so it is rejected, rolled back, and surfaced. See
[action-lifecycle.md](./action-lifecycle.md#transient-vs-permanent).

### Why flush one action at a time instead of batching?

Cribbage moves are order-dependent (a `play_card` before a `go`, discards before
pegging). `flush()` sends outstanding actions oldest-first, one at a time, and
stops the pass on the first transient failure so head-of-line ordering is
preserved. A permanent rejection is the exception — processing continues past it
because a later action may be independent.

### What is a "revision" and why is it just a count?

A revision is the number of durably-recorded moves for a game. It is monotonic
and cheap, and the client only ever *compares* revisions for ordering — it never
interprets the absolute value. That is all the engine needs to distinguish a
fresh snapshot from a stale one. See
[api-contract.md](./api-contract.md#revision-semantics).

### Why is the server's acceptance policy so coarse?

The server does not yet persist client ids alongside moves, so it cannot match a
`client_id` to a specific stored move. It uses a safe rule: if the client is
behind the server revision, its optimistic actions are superseded, so report
them all `accepted`; if the client is at the server revision, nothing new
landed, so keep them outstanding. The wire already carries the ids, so precise
per-action accept/reject is a non-breaking future change. See
[api-contract.md](./api-contract.md#acceptance-policy).

### What happens to inflight actions when the socket reconnects?

Their status is unknown — the request may or may not have reached the server. On
`ws_open` the engine reverts every `inflight` action back to `pending`
(`requeueInflight`) and triggers a resync. Re-offering is safe thanks to stable
client ids and reconciliation. See
[action-lifecycle.md](./action-lifecycle.md#reconnect-requeue).

### Does the client ever render the raw server snapshot?

No. The UI always renders `optimisticState`, which is the confirmed snapshot
with pending/inflight actions folded in. The raw `confirmedSnapshot` is only the
base for that computation. A `confirmed` action's effect comes from the snapshot
(not the fold) to avoid double-counting.

### Can one engine manage multiple games at once?

No. A `SyncEngine` is per-game: one instance per open game, keyed by `gameId`,
with its own `localStorage` key and its own WS room (`game:<gameId>`).
Cross-game coordination (e.g. a global lobby queue) is out of scope. See
[architecture.md](./architecture.md).

### What if the backend doesn't expose `/resync`?

The client degrades gracefully: `SyncEngine.resync` falls back to a plain
`GET /api/games/:id` refetch and reconciles against that snapshot with empty
`accepted`/`rejected` sets. The optimistic model still works end to end.
