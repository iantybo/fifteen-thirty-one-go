# The optimistic reducer

> Part of the [Optimistic Sync Engine](./README.md) docs.

The reducer is the "prediction" half of optimistic sync. Given the last
authoritative `CribbageState` and a `SyncAction`, it produces the state the
player *expects* to see immediately, before the server confirms anything.

## Contract

The reducer is **pure, total, and deterministic**:

- **Pure** — never mutates its input; returns a new state.
- **Total** — never throws. An action it cannot apply returns
  `{ state: <unchanged>, rejected: "<reason>" }`.
- **Deterministic** — no clock, no randomness. Same inputs → same output. This
  is what makes it testable and lets the engine re-fold the queue on any base
  snapshot during reconciliation.

## Module layout (`frontend/src/sync/reducers/`)

| File | Action |
| --- | --- |
| `shared.ts` | `ApplyResult`, `cloneState`, `cardToCode`, `cardValue15`, `nextActiveIndex`. |
| `play.ts` | `applyPlayCard` — pegging play. |
| `discard.ts` | `applyDiscard` — discard to crib. |
| `go.ts` | `applyGo` — pass during pegging. |
| `readyNextHand.ts` | `applyReadyNextHand` — toggle readiness during counting. |
| `index.ts` | barrel. |

`../reducer.ts` is a thin facade over these that preserves the original public
API (`applyAction`, `foldActions`).

## What it predicts — and what it does not

The client does not know the deck, the opponent's hand, or the exact scoring the
server will award. The reducer therefore predicts only **locally-derivable**
state:

| Action | Predicted | Left to the server |
| --- | --- | --- |
| `play_card` | card leaves hand, sequence grows, total += value, turn advances | pegging score (15s/pairs/runs/31) |
| `discard` | cards leave hand, `discard_completed[me]` = true | the cut, transition to pegging |
| `go` | `pegging_passed[me]` = true, turn advances | count reset, the "go" point |
| `ready_next_hand` | `ready_next_hand[me]` toggles | the actual next-hand deal |

Predicting an unverifiable score would guarantee a visible rollback. Letting the
peg board jump on reconciliation reads naturally as "the server awarded points."

## Folding

`foldActions(base, actions, myPos)` applies a list of actions in order on top of
`base`, skipping (not failing) any the reducer rejects. It returns the final
predicted state plus the `skipped` list. The engine uses it two ways:

1. compute `optimisticState` = `confirmedSnapshot.state` + pending queue, and
2. **rebase** the still-pending queue onto a fresh authoritative snapshot during
   reconciliation.

See [reconciliation.md](./reconciliation.md) for how rebasing folds into the
larger reconcile step.
