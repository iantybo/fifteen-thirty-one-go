# Reconciliation

> Part of the [Optimistic Sync Engine docs](./README.md). See also [action-lifecycle.md](./action-lifecycle.md), [glossary.md](./glossary.md).

Reconciliation is the step that pulls the optimistic client back toward
authoritative truth. Whenever a fresh server snapshot arrives — from the initial
fetch, a flush, a `game_update` WebSocket event, or a `resync` — the engine runs
`reconcile()` to decide the fate of every outstanding action and to recompute
the state the UI renders. The implementation is `SyncEngine.reconcile` in
`frontend/src/sync/engine.ts`.

## Inputs

```
reconcile(snapshot, incomingRevision, accepted[], rejected[])
```

| Input | Source | Meaning |
| --- | --- | --- |
| `snapshot` | `GameSnapshot` from HTTP/`resync` | The authoritative, user-specific game state. |
| `incomingRevision` | server revision (move count) | Used **only** for ordering — newer vs. stale. |
| `accepted` | resync response | Client ids the server has resolved; drop them. |
| `rejected` | resync response | Client ids the server refused; drop + surface. |

## The algorithm

```
                    ┌──────────────────────────────────────┐
                    │ incomingRevision <= revision          │
                    │ AND confirmedSnapshot != null ?        │
                    └───────────────┬──────────────────────┘
                          yes       │        no
                    ┌───────────────┘        └───────────────┐
                    ▼                                          ▼
          ┌───────────────────┐              ┌────────────────────────────────┐
          │ IGNORE (stale)    │              │ 1. queue.remove(accepted ∪       │
          │ ignoredStale=true │              │      rejected)                   │
          │ keep all pending  │              │ 2. confirmedSnapshot = snapshot  │
          └───────────────────┘              │    revision = incomingRevision   │
                                             │ 3. fold(snapshot.state,          │
                                             │      stillPending)               │
                                             │ 4. drop reducer-skipped actions  │
                                             │ 5. emit()                        │
                                             └────────────────────────────────┘
```

### Step 0 — Stale rejection via revisions

```ts
if (incomingRevision <= this.revision && this.confirmedSnapshot !== null) {
  // ignore: mark ignoredStale, keep every outstanding action pending
}
```

Revisions are the **count of durable moves** and are monotonically
non-decreasing. WebSocket deliveries can arrive out of order or be duplicated;
applying an older snapshot would visibly regress the board (peg jumps backward,
a played card reappears). The engine therefore compares revisions and drops any
snapshot that is not strictly newer than what it already has. The absolute value
is never interpreted — only the comparison matters. The very first snapshot is
always accepted because `confirmedSnapshot` is still `null`.

The result records `ignoredStale: true` and leaves the queue untouched, so a
stale delivery is a no-op from the queue's perspective.

### Step 1 — Confirm and reject (drop resolved ids)

Any client id the server reported as `accepted` or `rejected` is removed from
the durable queue:

```ts
this.queue.remove([...accepted, ...rejected])
```

- **Accepted** means "stop tracking this; the authoritative snapshot supersedes
  your optimistic copy." Its effect (if any) is already baked into `snapshot`.
- **Rejected** means "the server refused this move." The action is dropped and
  reported in `ReconcileResult.rejected` so the UI can tell the user. (Note that
  most permanent rejections are caught earlier, during `flush()`; see
  [action-lifecycle.md](./action-lifecycle.md).)

### Step 2 — Adopt the new base

```ts
this.confirmedSnapshot = snapshot
this.revision = incomingRevision
```

The new snapshot becomes the confirmed base from which optimistic state is
derived. Nothing the UI sees is derived from the raw snapshot directly — it is
always the *folded* result computed next.

### Step 3 — Fold pending onto the new base (rebase)

This is the heart of reconciliation. Rather than trying to surgically patch the
old optimistic state, the engine **re-derives** it from scratch: it takes the
fresh authoritative base and re-applies every still-pending action on top, in
author order, using the pure reducer.

```ts
const base = snapshot.state
const pending = this.queue.outstanding()
  .map((a) => ({ clientId: a.clientId, action: a.action }))
const { state, skipped } = foldActions(cloneState(base), pending, myPosition)
this.optimisticState = state
```

Because `foldActions` is pure, total, and deterministic, re-folding on any base
is safe and repeatable. This is what makes a **rebase** trivial: when the server
advances the game (e.g. the opponent moved), the client's still-outstanding
actions are simply replayed on top of the newer truth.

### Step 4 — Drop actions the reducer can no longer apply

`foldActions` returns a `skipped` list: actions the reducer could not apply to
the new base (for example, the server already advanced past a `go`, or a card
the action referenced is no longer in hand). These are treated as **implicitly
confirmed** — the server's snapshot already reflects their outcome — so they are
removed from the queue:

```ts
if (skipped.length > 0) {
  this.queue.remove(skipped.map((s) => s.clientId))
}
```

This is the automatic **rollback / cleanup** path: an optimistic action that no
longer makes sense against authoritative truth silently disappears rather than
wedging the queue.

## Confirm vs. rebase vs. rollback — summary

| Situation | Outcome |
| --- | --- |
| Server accepted the action (in `accepted`) | **Confirm**: dropped from queue; effect already in snapshot. |
| Server advanced but action still applies to new base | **Rebase**: re-folded on top of the new snapshot, stays pending. |
| Server refused the action (in `rejected` or `foldActions` skips it) | **Rollback**: dropped from queue; UI reverts to authoritative outcome. |
| Incoming snapshot older than current | **Ignore**: `ignoredStale`, queue untouched. |

## Output: `ReconcileResult`

Every pass returns a `ReconcileResult` (also stored as `lastReconcile` for
diagnostics):

```ts
type ReconcileResult = {
  confirmed: string[]     // ids the server accepted
  rejected: string[]      // ids the server rejected
  keptPending: string[]   // ids still outstanding after this pass
  ignoredStale: boolean   // true when the snapshot was stale and ignored
  revision: Revision      // the revision we reconciled to
}
```

The UI can use `rejected` to surface "that move was not allowed" and
`keptPending.length` to show an in-flight indicator. See
[glossary.md](./glossary.md) for the precise meaning of confirm, rebase, and
fold, and [api-contract.md](./api-contract.md) for how `accepted`/`rejected` are
produced server-side.
