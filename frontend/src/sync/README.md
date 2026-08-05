# `src/sync/` — Optimistic Sync Engine

A per-game controller that applies cribbage actions **optimistically** (via a
pure reducer), queues them **durably** (localStorage) with retry/backoff, flushes
them to the server, and **reconciles** against authoritative snapshots delivered
over HTTP or via WebSocket `game_update` events.

The board moves on the next frame; the network round-trip is off the critical
path; actions survive reloads and disconnects.

For the full narrative and design rationale, see the docs:

- Design doc: [`../../../docs/optimistic-sync-engine.md`](../../../docs/optimistic-sync-engine.md)
- Deep-dive docs index: [`../../../docs/sync/`](../../../docs/sync/README.md)

## Quick start

Use the React hook — it constructs, `start()`s, subscribes to, and `stop()`s the
engine for you across the component lifecycle:

```tsx
import { useSyncEngine } from './sync'

function GamePage({ gameId, userId }: { gameId: number; userId: number }) {
  const { state, dispatch, connected } = useSyncEngine(gameId, userId)

  // Render the OPTIMISTIC state, never the raw server snapshot.
  const game = state.optimisticState
  if (!game) return <Spinner />

  return (
    <Board
      state={game}
      connected={connected}
      onPlayCard={(card) => dispatch({ kind: 'play_card', card })}
      onDiscard={(cards) => dispatch({ kind: 'discard', cards })}
      onGo={() => dispatch({ kind: 'go' })}
      onReadyNextHand={() => dispatch({ kind: 'ready_next_hand' })}
    />
  )
}
```

`dispatch(action)` returns the generated `clientId` so callers can correlate a
later rejection (surfaced via `state.lastReconcile.rejected`).

### Direct engine use (outside React / in tests)

```ts
import { SyncEngine } from './sync/engine'

const engine = new SyncEngine(gameId, userId, {
  // all optional — see EngineDeps
  now: () => Date.now(),
  setTimer: (fn, ms) => { const h = setTimeout(fn, ms); return () => clearTimeout(h) },
  store: myKeyValueStore,
  makeWs: () => new WsClient(),
  gameApi: api,
})

const off = engine.subscribe((s) => render(s))
await engine.start()            // connect WS + fetch first snapshot
engine.dispatch({ kind: 'go' }) // optimistic + enqueue + flush
// ...
off()
engine.stop()
```

## Module map

| File | Responsibility |
| --- | --- |
| `types.ts` | Shared, framework-agnostic types + the module design narrative. |
| `reducer.ts` | Pure, total, deterministic optimistic reducer (`applyAction`, `foldActions`, `cloneState`). Predicts turn/pegging/hand — never scores. |
| `queue.ts` | Durable, ordered, persisted action queue with backoff bookkeeping (`ActionQueue`, `KeyValueStore`). Degrades to memory. |
| `backoff.ts` | Pure exponential backoff schedule (`backoffDelayMs`). |
| `errors.ts` | Transient-vs-permanent error classification (`isPermanentError`, `errorMessage`). |
| `clientId.ts` | Deterministic, monotonic client ids `c:<gameId>:<userId>:<seq>` (`IdGenerator`). |
| `engine.ts` | Orchestrator: dispatch → optimistic → enqueue → flush → reconcile (`SyncEngine`, `EngineDeps`, `toWireMove`). |
| `useSyncEngine.ts` | React binding hook. |
| `index.ts` | Public barrel. |

## Key invariants

- The UI renders `optimisticState` (confirmed base + folded pending), never the
  raw snapshot.
- The reducer is **pure/total/deterministic** — no clock, no randomness, never
  throws, never mutates inputs.
- Client ids are **stable across retries** so re-offers dedupe.
- Revisions are **compared, never interpreted** — ordering only.
- Storage failures **degrade to memory**, never crash.

## Testing

Every non-deterministic dependency is injected via `EngineDeps`
(`now`, `setTimer`, `store`, `makeWs`, `gameApi`), so the engine runs under a
fake clock, fake WS, stub API, and in-memory store. See
[`../../../docs/sync/testing-strategy.md`](../../../docs/sync/testing-strategy.md).

```bash
cd frontend && npm test           # reducer.test.ts, queue.test.ts, engine.test.ts
```

## Backend counterpart

`POST /api/games/:id/resync` — reconciles the outstanding queue against
authoritative state. Contract mirrored by `ResyncResponse` in `types.ts`; see
[`../../../docs/sync/api-contract.md`](../../../docs/sync/api-contract.md) and
`backend/internal/handlers/resync.go`.
