# API Contract — `POST /api/games/:id/resync`

> Part of the [Optimistic Sync Engine docs](./README.md). See also [reconciliation.md](./reconciliation.md), [glossary.md](./glossary.md).

The resync endpoint reconciles a client's outstanding action queue against the
authoritative server state and returns the current snapshot plus the
accepted/rejected client ids. It is implemented in
`backend/internal/handlers/resync.go` and mirrored on the client by
`ResyncResponse` in `frontend/src/sync/types.ts`.

```
POST /api/games/:id/resync
```

The endpoint is **optional** for the client: if a deployed backend does not
expose it, the engine degrades to a plain `GET /api/games/:id` refetch and
reconciles against that (`SyncEngine.resync` → `refetchAndReconcile`).

## Request

| Field | Type | Meaning |
| --- | --- | --- |
| `last_revision` | number | The client's last-known server revision (a move count). |
| `pending` | array | The client's outstanding queue: `{ client_id, kind }` per action. |
| `pending[].client_id` | string | Stable client id, format `c:<gameId>:<userId>:<seq>`. |
| `pending[].kind` | string | Action kind (`discard` / `play_card` / `go` / `ready_next_hand`). |

```json
{
  "last_revision": 12,
  "pending": [
    { "client_id": "c:42:7:5", "kind": "play_card" },
    { "client_id": "c:42:7:6", "kind": "go" }
  ]
}
```

## Response (`ResyncResponse`)

| Field | Type | Meaning |
| --- | --- | --- |
| `snapshot` | `GameSnapshot` | The authoritative, user-specific snapshot. |
| `revision` | number | The server's current revision for this game. |
| `accepted` | string[] | Client ids the server considers resolved — the client stops tracking them. Always `[]`, never `null`. |
| `rejected` | string[] | Client ids the server refused. Always `[]`, never `null`. |

```json
{
  "snapshot": { "game_id": 42, "state": { "...": "..." }, "players": [] },
  "revision": 13,
  "accepted": ["c:42:7:5", "c:42:7:6"],
  "rejected": []
}
```

Note both slices are always non-nil (`make([]string, 0, …)`) so the JSON
serializes to `[]` rather than `null`. The client treats `null` and `[]`
identically, but `[]` is friendlier for consumers and tests.

## Revision semantics

A game's **revision** is defined as the count of durably-recorded moves
(`gameRevision` → `models.ListMovesByGame(db, gameID, -1)`). Properties:

- **Monotonically non-decreasing** — it only advances as moves are persisted.
- **Cheap to compute** — a count, no diffing.
- **Ordering-only** — the client only ever compares revisions (newer vs. stale);
  it never interprets the absolute value. See the stale-rejection rule in
  [reconciliation.md](./reconciliation.md).

## Acceptance policy

The server does not yet persist client action ids alongside moves, so it cannot
match a specific `client_id` to a specific stored move. It uses a conservative,
safe rule instead (`reconcilePending`):

```
if last_revision < serverRevision:
    # server advanced past the client's optimistic actions:
    # they were applied or superseded — client should stop replaying
    accepted = every pending client_id (with a non-empty id)
    rejected = pending entries with an empty client_id (defensive)
else:  # last_revision >= serverRevision
    # nothing new landed — actions genuinely still outstanding
    accepted = []
    rejected = []
```

| Case | `accepted` | `rejected` | Client effect |
| --- | --- | --- | --- |
| `last_revision < revision` | all pending ids | ids with empty `client_id` | Drop accepted; re-fold remaining; roll back rejected. |
| `last_revision >= revision` | `[]` | `[]` | Keep replaying pending actions. |
| Pending entry with empty `client_id` | — | that entry | Defensive: prompt client to drop an unreconcilable entry. |

The wire contract already carries the ids, so a future change to precise
per-action accept/reject is **non-breaking**.

## Status codes

| Code | Condition |
| --- | --- |
| `200 OK` | Reconciled; body is `resyncResponse`. |
| `400 Bad Request` | Invalid/non-positive game id, or invalid JSON body. |
| `401 Unauthorized` | No authenticated user in context. |
| `403 Forbidden` | Authenticated user is not a participant in the game. |
| `404 Not Found` | Game (or its snapshot) does not exist. |
| `409 Conflict` | Game state missing/unavailable (`recreate lobby`). |
| `500 Internal Server Error` | DB error computing the revision or building the snapshot. |

### Example — client behind server (`200`)

Request `last_revision: 12` against a server at revision `13`:

```json
{
  "snapshot": { "game_id": 42, "state": {}, "players": [] },
  "revision": 13,
  "accepted": ["c:42:7:5", "c:42:7:6"],
  "rejected": []
}
```

### Example — client up to date (`200`)

Request `last_revision: 13` against a server at revision `13`:

```json
{
  "snapshot": { "game_id": 42, "state": {}, "players": [] },
  "revision": 13,
  "accepted": [],
  "rejected": []
}
```

### Example — not a participant (`403`)

```json
{ "error": "not a player" }
```

## Authorization

Only participants may resync a game, preventing snapshot leakage to arbitrary
users. The handler enforces this with `models.IsUserInGame(db, userID, gameID)`,
mirroring the guards in `QuitGameHandler` / `NextHandHandler`.
