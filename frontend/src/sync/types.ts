/**
 * Optimistic Sync Engine — shared types.
 *
 * ## Why this module exists
 *
 * Before the sync engine, every game action in the client followed a strict
 * request→await→refetch pattern (see the pre-existing `submitMove` in
 * `pages/GamePage.tsx`): the UI disabled itself, POSTed the move, waited for the
 * server, then re-fetched the whole snapshot. That is simple but has three
 * problems the sync engine is designed to fix:
 *
 *  1. **Perceived latency.** The board does not move until the server responds.
 *     On a slow link every discard/peg feels sluggish.
 *  2. **No offline tolerance.** If the socket drops mid-hand, actions are simply
 *     lost and the player sees an error.
 *  3. **Full-snapshot churn.** Every action triggers a complete `GET /games/:id`
 *     even when only the pegging total changed.
 *
 * The sync engine replaces that model with **optimistic application +
 * reconciliation**:
 *
 *  - Actions are applied *locally and immediately* via a pure reducer
 *    (`reducer.ts`) so the UI updates on the next frame.
 *  - Actions are appended to a durable queue (`queue.ts`) that survives reloads
 *    and disconnects, and is flushed to the server with retry/backoff.
 *  - The authoritative server snapshot is treated as the source of truth: when
 *    it arrives (via HTTP response or WS `game_update`), the engine
 *    *reconciles* — confirming, rebasing, or rolling back optimistic actions.
 *
 * These types are deliberately framework-agnostic (no React imports) so the
 * reducer and queue can be unit-tested in isolation.
 */

import type { CribbageState, GameSnapshot } from '../api/types'

/**
 * A monotonically increasing revision number the engine attaches to every
 * locally-known state. The server's authoritative revision is compared against
 * this to decide whether an incoming snapshot is newer, older, or concurrent.
 *
 * Revisions are per-game and start at 0 for a freshly-fetched snapshot.
 */
export type Revision = number

/**
 * The set of game actions the engine can apply optimistically. This mirrors
 * `GameMoveRequest` in `api/client.ts` but is a distinct type on purpose: the
 * engine layer owns its own action vocabulary and maps to the wire format at
 * the boundary (see `engine.ts#toWireMove`). Keeping them separate means the
 * reducer never depends on transport details.
 */
export type SyncAction =
  | { kind: 'discard'; cards: string[] }
  | { kind: 'play_card'; card: string }
  | { kind: 'go' }
  | { kind: 'ready_next_hand' }

/**
 * Lifecycle state of a queued action.
 *
 * ```
 *  pending ──flush()──▶ inflight ──ack──▶ confirmed
 *     ▲                    │
 *     └──────retry─────────┘ (on transient failure)
 *                          │
 *                          └──reject──▶ rejected  (on 4xx / reconciliation mismatch)
 * ```
 */
export type QueuedActionStatus = 'pending' | 'inflight' | 'confirmed' | 'rejected'

/**
 * A single action tracked by the durable queue. `clientId` is generated on the
 * client and is the stable identity used everywhere (dedupe, reconciliation,
 * rollback). The server echoes it back in the resync response so we can match
 * acknowledgements to the originating action even when responses arrive out of
 * order.
 */
export type QueuedAction = {
  /** Stable client-generated id (see `engine.ts#nextClientId`). */
  clientId: string
  /** The game this action belongs to. */
  gameId: number
  /** The optimistic action payload. */
  action: SyncAction
  /** Current lifecycle status. */
  status: QueuedActionStatus
  /**
   * The engine revision the action was created against. Used to detect when an
   * action was authored on top of state the server has since superseded (a
   * "stale base"), which forces a rebase rather than a naive confirm.
   */
  baseRevision: Revision
  /** Monotonic sequence used to preserve author order across reloads. */
  seq: number
  /** Wall-clock millisecond timestamp of creation (for backoff + telemetry). */
  createdAt: number
  /** Number of delivery attempts so far (drives exponential backoff). */
  attempts: number
  /** Last error message, if the most recent attempt failed. */
  lastError?: string
}

/**
 * The engine's full view of a single game. This is what the React hook renders
 * from — never the raw server snapshot directly, because `optimisticState`
 * folds pending actions on top of `confirmedSnapshot`.
 */
export type EngineGameState = {
  gameId: number
  /** The last authoritative snapshot received from the server. */
  confirmedSnapshot: GameSnapshot | null
  /**
   * `confirmedSnapshot.state` with every still-pending/inflight action folded
   * in via the reducer. This is what the UI should render.
   */
  optimisticState: CribbageState | null
  /** Engine revision of `confirmedSnapshot`. */
  revision: Revision
  /** Actions not yet confirmed by the server, in author order. */
  queue: QueuedAction[]
  /** Whether the transport currently believes it is connected. */
  connected: boolean
  /** True while a reconciliation pass is in flight. */
  reconciling: boolean
  /** Last reconciliation outcome, for diagnostics/telemetry. */
  lastReconcile?: ReconcileResult
}

/**
 * Result of reconciling local optimistic state against an authoritative
 * snapshot. Exposed so the UI can surface "N actions rolled back" style
 * feedback and so tests can assert precisely what the engine decided.
 */
export type ReconcileResult = {
  /** clientIds the server accepted (fold complete; remove from queue). */
  confirmed: string[]
  /** clientIds the server rejected (roll back; surface to user). */
  rejected: string[]
  /** clientIds still pending after reconciliation (kept in queue). */
  keptPending: string[]
  /**
   * True when the incoming snapshot's revision was **older** than what we
   * already had (a late/duplicate delivery); the engine ignores such snapshots
   * to avoid regressing the board. See `engine.ts#reconcile`.
   */
  ignoredStale: boolean
  /** The revision we reconciled to. */
  revision: Revision
}

/**
 * The wire shape of the backend resync endpoint response
 * (`POST /api/games/:id/resync`). Kept in sync with `resync.go`.
 */
export type ResyncResponse = {
  /** Authoritative snapshot for the requesting user. */
  snapshot: GameSnapshot
  /** Server's current revision for this game. */
  revision: Revision
  /** clientIds the server has durably applied. */
  accepted: string[]
  /** clientIds the server refused (illegal/duplicate/stale). */
  rejected: string[]
}

/** Opaque unsubscribe handle returned by the engine's `subscribe`. */
export type Unsubscribe = () => void

/** Listener invoked whenever an `EngineGameState` changes. */
export type EngineListener = (state: EngineGameState) => void
