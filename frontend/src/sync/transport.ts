/**
 * Transport seam for the Optimistic Sync Engine.
 *
 * ## Why the engine depends on an interface, not on `api` directly
 *
 * The engine's flush/reconcile loop only needs three network operations:
 * fetch a game snapshot, submit a move, and advance to the next hand. Today
 * those are backed by the concrete `api` object in `../api/client`, but the
 * engine should not *know* that. Depending on a narrow {@link GameTransport}
 * interface instead of the concrete `api` buys two things:
 *
 *  1. **Testability.** The flush loop has genuinely tricky behavior — ordering,
 *     backoff, optimistic rollback — that we want to exercise against stubbed
 *     responses (resolve, reject with a 5xx, reject with a 4xx, hang) without a
 *     real server or `fetch`. A hand-written fake implementing this three-method
 *     interface is trivial to write; stubbing the full `api` surface is not.
 *
 *  2. **Dependency inversion.** The engine declares the (small) capability it
 *     requires and lets a `RealTransport` adapter supply it. Swapping in a
 *     replay transport, an in-memory transport for local play, or a
 *     rate-limiting decorator is then an injection detail, not an engine change.
 *
 * `RealTransport` is the production adapter that simply delegates to `api`. It
 * carries no logic of its own on purpose: all the delegation lives in one thin
 * place so the interface and the concrete client cannot drift apart unnoticed.
 */

import { api, type GameMoveRequest } from '../api/client'
import type { GameSnapshot } from '../api/types'

/**
 * The minimal network capability the sync engine requires. Implementations may
 * be the real HTTP client, a test fake, or any other adapter.
 */
export interface GameTransport {
  /** Fetch the authoritative snapshot for a game. */
  getGame(gameId: number): Promise<GameSnapshot>
  /** Submit a move; resolves on acceptance, rejects (e.g. `ApiError`) on refusal. */
  moveGame(gameId: number, move: GameMoveRequest): Promise<unknown>
  /** Signal readiness to advance to the next hand. */
  nextHand(gameId: number): Promise<void>
}

/**
 * Production {@link GameTransport} that delegates to the concrete `api` client.
 *
 * Intentionally logic-free: it exists only to adapt the wide `api` object down
 * to the narrow interface the engine depends on.
 */
export class RealTransport implements GameTransport {
  getGame(gameId: number): Promise<GameSnapshot> {
    return api.getGame(gameId)
  }

  moveGame(gameId: number, move: GameMoveRequest): Promise<unknown> {
    return api.moveGame(gameId, move)
  }

  nextHand(gameId: number): Promise<void> {
    return api.nextHand(gameId)
  }
}
