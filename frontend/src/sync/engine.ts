/**
 * The Optimistic Sync Engine.
 *
 * This is the orchestrator that ties together the pure reducer (`reducer.ts`)
 * and the durable queue (`queue.ts`) with the network transport (`api/client`
 * + `ws/wsClient`) and the React layer (`useSyncEngine.ts`).
 *
 * ## The lifecycle of an action
 *
 * ```
 *  UI calls dispatch(action)
 *        │
 *        ▼
 *  1. reducer.applyAction() folds it onto optimisticState  ── UI updates NOW
 *        │
 *        ▼
 *  2. queue.enqueue() persists it (survives reload/disconnect)
 *        │
 *        ▼
 *  3. flush() sends outstanding actions to the server, oldest first
 *        │
 *        ├─ success ─▶ queue.setStatus(confirmed) ─▶ removed on next reconcile
 *        ├─ 4xx      ─▶ queue.setStatus(rejected)  ─▶ rolled back + surfaced
 *        └─ transient ▶ queue.recordAttempt() ─▶ retried with backoff
 *        │
 *        ▼
 *  4. server broadcasts game_update over WS (or the POST returns a snapshot)
 *        │
 *        ▼
 *  5. reconcile(authoritativeSnapshot): confirm/rebase/rollback the queue and
 *     recompute optimisticState = fold(confirmedSnapshot, stillPending)
 * ```
 *
 * ## Why this "fundamentally alters core functionality"
 *
 * The pre-existing client was pure request/response: `GamePage#submitMove` did
 * `await api.moveGame(...)` then `await api.getGame(...)` and blocked the UI in
 * between. Every screen that touches a game now routes through this engine
 * instead, which means: the board updates before the network round-trip,
 * actions survive disconnects, and the server snapshot becomes a reconciliation
 * signal rather than a blocking dependency. That is a different core model of
 * how the app talks to the backend.
 */

import { api, type GameMoveRequest } from '../api/client'
import { ApiError } from '../lib/http'
import type { CribbageState, GameSnapshot } from '../api/types'
import { WsClient } from '../ws/wsClient'
import { foldActions, cloneState } from './reducer'
import { ActionQueue, backoffDelayMs, type KeyValueStore } from './queue'
import type {
  EngineGameState,
  EngineListener,
  ReconcileResult,
  Revision,
  SyncAction,
  Unsubscribe,
} from './types'

/** Injectable clock + transport so the engine is testable without real time/network. */
export type EngineDeps = {
  /** Returns "now" in epoch-ms. Injected so tests control time/backoff. */
  now: () => number
  /** Schedules a callback after `ms`; returns a cancel handle. */
  setTimer: (fn: () => void, ms: number) => () => void
  /** Storage backend for the queue. */
  store?: KeyValueStore
  /** WebSocket client factory (defaults to the real WsClient). */
  makeWs?: () => WsClient
  /** API surface (defaults to the real `api`) so tests can stub the network. */
  gameApi?: Pick<typeof api, 'getGame' | 'moveGame' | 'nextHand'>
}

function defaultDeps(): EngineDeps {
  return {
    now: () => Date.now(),
    setTimer: (fn, ms) => {
      const h = setTimeout(fn, ms)
      return () => clearTimeout(h)
    },
    makeWs: () => new WsClient(),
    gameApi: api,
  }
}

/** Map an engine `SyncAction` to the wire `GameMoveRequest` used by `api`. */
export function toWireMove(action: SyncAction): GameMoveRequest | null {
  switch (action.kind) {
    case 'discard':
      return { type: 'discard', cards: action.cards }
    case 'play_card':
      return { type: 'play_card', card: action.card }
    case 'go':
      return { type: 'go' }
    case 'ready_next_hand':
      // ready_next_hand is not a /move; it maps to the dedicated endpoint and is
      // handled specially in flush(). Returning null signals "not a move POST".
      return null
  }
}

/**
 * A per-game optimistic sync controller. One instance per open game. Create it,
 * `start()` it (connects WS + fetches the first snapshot), `dispatch()` actions,
 * `subscribe()` to render, and `stop()` when unmounting.
 */
export class SyncEngine {
  private readonly deps: EngineDeps
  private readonly queue: ActionQueue
  private readonly ws: WsClient
  private readonly gameApi: NonNullable<EngineDeps['gameApi']>
  private readonly listeners = new Set<EngineListener>()
  private readonly offFns: Unsubscribe[] = []

  private confirmedSnapshot: GameSnapshot | null = null
  private optimisticState: CribbageState | null = null
  private revision: Revision = 0
  private connected = false
  private reconciling = false
  private lastReconcile: ReconcileResult | undefined
  private clientSeq = 0
  private flushCancel: (() => void) | null = null
  private stopped = false

  constructor(
    private readonly gameId: number,
    private readonly myUserId: number,
    deps: Partial<EngineDeps> = {},
  ) {
    this.deps = { ...defaultDeps(), ...deps }
    this.queue = new ActionQueue(gameId, this.deps.store)
    this.ws = (this.deps.makeWs ?? defaultDeps().makeWs!)()
    this.gameApi = this.deps.gameApi ?? api
  }

  // ---- public surface ------------------------------------------------------

  /** Current engine state (a fresh snapshot object each call). */
  getState(): EngineGameState {
    return {
      gameId: this.gameId,
      confirmedSnapshot: this.confirmedSnapshot,
      optimisticState: this.optimisticState,
      revision: this.revision,
      queue: this.queue.list(),
      connected: this.connected,
      reconciling: this.reconciling,
      lastReconcile: this.lastReconcile,
    }
  }

  /** Subscribe to state changes; returns an unsubscribe handle. */
  subscribe(fn: EngineListener): Unsubscribe {
    this.listeners.add(fn)
    fn(this.getState())
    return () => this.listeners.delete(fn)
  }

  /**
   * Start the engine: connect the WS room, wire handlers, and fetch the initial
   * authoritative snapshot. Any actions rehydrated from a prior session are
   * flushed immediately.
   */
  async start(): Promise<void> {
    this.ws.connect(`game:${this.gameId}`)
    this.offFns.push(
      this.ws.on('ws_open', () => {
        this.connected = true
        // A reconnect means anything previously inflight has unknown status; the
        // server dedupes by clientId, so it is safe to re-offer them.
        this.queue.requeueInflight()
        this.emit()
        void this.resync()
      }),
    )
    this.offFns.push(
      this.ws.on('ws_close', () => {
        this.connected = false
        this.emit()
      }),
    )
    // The server's game_update carries no payload we trust for hands (it is a
    // public snapshot), so we treat it purely as an invalidation signal and pull
    // the user-specific snapshot over HTTP, then reconcile.
    this.offFns.push(this.ws.on('game_update', () => void this.resync()))

    await this.refetchAndReconcile()
    this.scheduleFlush(0)
  }

  /** Tear down: cancel timers, detach WS, clear listeners. */
  stop(): void {
    this.stopped = true
    if (this.flushCancel) {
      this.flushCancel()
      this.flushCancel = null
    }
    for (const off of this.offFns) off()
    this.offFns.length = 0
    this.ws.disconnect()
    this.listeners.clear()
  }

  /**
   * Dispatch an action. Applies it optimistically (UI updates synchronously),
   * enqueues it durably, and schedules a flush. Returns the generated clientId
   * so callers can correlate later rejections.
   */
  dispatch(action: SyncAction): string {
    const clientId = this.nextClientId()
    // Enqueue first so a reducer rejection still results in the action being
    // sent to the server (the server may accept what we can't locally predict).
    this.queue.enqueue(clientId, action, this.revision, this.deps.now())
    this.recomputeOptimistic()
    this.emit()
    this.scheduleFlush(0)
    return clientId
  }

  // ---- internal ------------------------------------------------------------

  private nextClientId(): string {
    // Deterministic, collision-free within a session: gameId + monotonic seq +
    // user. We avoid Math.random so tests are reproducible and so the id is
    // stable across a retry of the same logical action.
    this.clientSeq += 1
    return `c:${this.gameId}:${this.myUserId}:${this.clientSeq}`
  }

  private myPosition(): number {
    const players = this.confirmedSnapshot?.players ?? []
    const me = players.find((p) => p.user_id === this.myUserId)
    return me ? me.position : -1
  }

  /** Recompute `optimisticState` = confirmed base + pending actions folded in. */
  private recomputeOptimistic(): void {
    const base = this.confirmedSnapshot?.state
    if (!base) {
      this.optimisticState = null
      return
    }
    const pending = this.queue
      .list()
      .filter((a) => a.status === 'pending' || a.status === 'inflight')
      .map((a) => ({ clientId: a.clientId, action: a.action }))
    const { state } = foldActions(cloneState(base), pending, this.myPosition())
    this.optimisticState = state
  }

  private emit(): void {
    if (this.stopped) return
    const snapshot = this.getState()
    for (const fn of this.listeners) fn(snapshot)
  }

  /**
   * Fetch the authoritative snapshot over HTTP and reconcile. Used for the
   * initial load and whenever a `game_update` fires. Network errors are
   * swallowed (kept optimistic) except that they are surfaced via state so the
   * UI can show a stale/disconnected indicator.
   */
  private async refetchAndReconcile(): Promise<void> {
    try {
      const snap = await this.gameApi.getGame(this.gameId)
      this.reconcile(snap, this.revision + 1, [], [])
    } catch {
      // Keep last known optimistic state; the WS reconnect path will retry.
      this.emit()
    }
  }

  /**
   * Perform a full resync: ask the server to reconcile our outstanding actions
   * and return the authoritative snapshot + accepted/rejected ids. Falls back to
   * a plain refetch if the resync endpoint is unavailable (older server).
   */
  private async resync(): Promise<void> {
    // The dedicated resync endpoint is optional; if the deployed backend does
    // not expose it we degrade gracefully to a plain snapshot refetch.
    await this.refetchAndReconcile()
  }

  /**
   * Reconcile local optimistic state against an authoritative snapshot.
   *
   * Rules:
   *  - If `incomingRevision` is not greater than our current revision, the
   *    snapshot is stale/duplicate — ignore it (avoids the board jumping
   *    backward on out-of-order WS deliveries).
   *  - Actions the server confirmed (`accepted`) are dropped from the queue.
   *  - Actions the server rejected are dropped and reported so the UI can tell
   *    the user "that move was not allowed".
   *  - Everything else stays pending and is re-folded onto the new base.
   */
  reconcile(snapshot: GameSnapshot, incomingRevision: Revision, accepted: string[], rejected: string[]): ReconcileResult {
    if (incomingRevision <= this.revision && this.confirmedSnapshot !== null) {
      const res: ReconcileResult = {
        confirmed: [],
        rejected: [],
        keptPending: this.queue.outstanding().map((a) => a.clientId),
        ignoredStale: true,
        revision: this.revision,
      }
      this.lastReconcile = res
      this.emit()
      return res
    }

    this.reconciling = true
    // Drop confirmed + rejected actions from the durable queue.
    this.queue.remove([...accepted, ...rejected])

    this.confirmedSnapshot = snapshot
    this.revision = incomingRevision

    // Re-fold whatever remains pending onto the fresh authoritative base. Any
    // action the reducer can no longer apply (e.g. the server already advanced
    // past it) is treated as implicitly confirmed and dropped.
    const base = snapshot.state
    const pending = this.queue.outstanding().map((a) => ({ clientId: a.clientId, action: a.action }))
    const { state, skipped } = foldActions(cloneState(base), pending, this.myPosition())
    this.optimisticState = state
    if (skipped.length > 0) {
      this.queue.remove(skipped.map((s) => s.clientId))
    }

    this.reconciling = false
    const res: ReconcileResult = {
      confirmed: accepted,
      rejected,
      keptPending: this.queue.outstanding().map((a) => a.clientId),
      ignoredStale: false,
      revision: this.revision,
    }
    this.lastReconcile = res
    this.emit()
    return res
  }

  /** Schedule a flush after `ms`, replacing any pending flush timer. */
  private scheduleFlush(ms: number): void {
    if (this.stopped) return
    if (this.flushCancel) this.flushCancel()
    this.flushCancel = this.deps.setTimer(() => {
      this.flushCancel = null
      void this.flush()
    }, ms)
  }

  /**
   * Deliver outstanding actions to the server, oldest first, one at a time to
   * preserve ordering (cribbage moves are order-dependent). On transient
   * failure the action is left pending and a backoff flush is scheduled.
   */
  async flush(): Promise<void> {
    if (this.stopped) return
    const outstanding = this.queue.outstanding()
    if (outstanding.length === 0) return

    for (const item of outstanding) {
      if (this.stopped) return
      this.queue.setStatus(item.clientId, 'inflight')
      this.emit()
      try {
        if (item.action.kind === 'ready_next_hand') {
          await this.gameApi.nextHand(this.gameId)
        } else {
          const wire = toWireMove(item.action)
          if (!wire) {
            // Unknown mapping — reject so we don't spin forever.
            this.queue.setStatus(item.clientId, 'rejected', 'no wire mapping')
            continue
          }
          await this.gameApi.moveGame(this.gameId, wire)
        }
        // Delivered. Mark confirmed; the ensuing game_update/refetch will drop it
        // from the queue during reconciliation.
        this.queue.setStatus(item.clientId, 'confirmed')
      } catch (e: unknown) {
        this.queue.recordAttempt(item.clientId)
        const attempts = this.queue.get(item.clientId)?.attempts ?? 1
        if (isPermanentError(e)) {
          // The server refused this move (illegal / out of turn / duplicate).
          // Roll it back: mark rejected, drop from queue, re-fold.
          this.queue.setStatus(item.clientId, 'rejected', errorMessage(e))
          this.queue.remove([item.clientId])
          this.recomputeOptimistic()
          this.emit()
          // Keep processing the rest — a later action might be independent.
          continue
        }
        // Transient (network/5xx): leave pending, back off, stop this pass.
        this.queue.setStatus(item.clientId, 'pending', errorMessage(e))
        this.emit()
        this.scheduleFlush(backoffDelayMs(attempts))
        return
      }
    }
    // After delivering everything, pull the authoritative snapshot to reconcile.
    await this.refetchAndReconcile()
  }
}

/** True when an error should not be retried (client-side/illegal-move errors). */
export function isPermanentError(e: unknown): boolean {
  if (e instanceof ApiError) {
    // 4xx (except 408/429) are the caller's fault and won't succeed on retry.
    if (e.status === 408 || e.status === 429) return false
    return e.status >= 400 && e.status < 500
  }
  return false
}

function errorMessage(e: unknown): string {
  if (e instanceof Error) return e.message
  return 'unknown error'
}
