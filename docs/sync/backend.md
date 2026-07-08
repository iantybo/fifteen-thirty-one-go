# Backend: resync endpoint

> Part of the [Optimistic Sync Engine](./README.md) docs. Frontend counterpart:
> [architecture.md](./architecture.md).

The backend's role in optimistic sync is small but load-bearing: it remains the
**single source of truth**. The client predicts; the server adjudicates. The
`POST /api/games/:id/resync` endpoint is how the client asks the server to
reconcile its optimistic action queue against authoritative state.

## File layout (`backend/internal/handlers/`)

| File | Responsibility |
| --- | --- |
| `resync.go` | `ResyncHandler` — request parsing, auth, orchestration. |
| `resync_types.go` | Wire types (`resyncRequest`, `resyncResponse`, `resyncPendingAction`). |
| `resync_revision.go` | `gameRevision` + `revisionIsNewer` / `revisionIsStale`. |
| `resync_policy.go` | `reconcilePending` + `classifyPending` (the acceptance policy). |
| `resync_response.go` | `ensureNonNilIDs`, `dedupeIDs`, `partitionPending`. |
| `client_id.go` | `parseClientID` / `isValidClientID` (shared format with the client). |

Each has a colocated `*_test.go` with table-driven, `t.Parallel()` tests.

## Request / response

```jsonc
// POST /api/games/42/resync
{
  "last_revision": 3,
  "pending": [
    { "client_id": "c:42:9:1", "kind": "play_card" },
    { "client_id": "c:42:9:2", "kind": "go" }
  ]
}
```

```jsonc
// 200 OK
{
  "snapshot": { "game": { /* … */ }, "players": [ /* … */ ], "state": { /* … */ } },
  "revision": 5,
  "accepted": ["c:42:9:1", "c:42:9:2"],
  "rejected": []
}
```

## Revision

A game's revision is the count of durably-recorded moves (`gameRevision`). It is
monotonically non-decreasing and cheap to compute. Ordering is compared with
`revisionIsNewer` / `revisionIsStale`, which partition the space (every pair of
revisions is exactly one of newer / stale relative to a baseline). See
[reconciliation.md](./reconciliation.md).

## Acceptance policy

`reconcilePending` (in `resync_policy.go`) is a pure function of the request and
the server revision, so it is unit-tested without a database:

- **Client behind** (`last_revision < serverRevision`): the client's optimistic
  actions have been superseded by the authoritative snapshot → reported
  `accepted` (client stops tracking them). A blank `client_id` is `rejected`.
- **Client up to date / ahead**: nothing to reconcile; actions stay outstanding.

The wire contract already carries `client_id`, so a future change to precise
per-action acceptance (matching stored moves to client ids) is non-breaking.

## Client-id contract

`client_id.go#parseClientID` mirrors the client's format `c:<gameId>:<userId>:<seq>`
(see `frontend/src/sync/clientId.ts`). Note the Go parser uses base-10
`strconv.ParseInt` and is therefore *stricter* than the JS `Number(...)` parser;
this is a safe tightening because real ids are always base-10 integer counters.

## Security

`ResyncHandler` requires the caller to be a game participant
(`models.IsUserInGame`) before returning any snapshot — the same guard used by
`QuitGameHandler` and `NextHandHandler`. This prevents leaking a user-specific
snapshot (which includes the requester's hand) to non-participants.
