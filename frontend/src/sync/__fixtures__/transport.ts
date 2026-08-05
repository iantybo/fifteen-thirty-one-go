/**
 * A controllable fake of the engine's HTTP transport (`EngineDeps.gameApi`).
 *
 * The engine only touches three methods of the real `api` object —
 * `getGame`, `moveGame`, `nextHand` — so this double implements exactly those
 * and nothing else. It is deliberately imperative and inspectable rather than
 * built on `vi.fn`, because the integration tests want to assert on *ordering*
 * and *arguments across many calls*, which reads more clearly against plain
 * recorded arrays than against mock matchers.
 *
 * Capabilities:
 *  - `snapshot` — the `GameSnapshot` returned by `getGame`. Reassign it between
 *    calls to simulate the server advancing (the engine reconciles to it).
 *  - `getGameCalls` / `moveGameCalls` / `nextHandCalls` — recorded argument
 *    tuples, in call order, for assertions and counting.
 *  - `failMoveWith(err)` / `clearMoveFailure()` — make the next (and subsequent)
 *    `moveGame` calls reject with `err`, simulating a transient network error or
 *    a permanent 4xx. Recording still happens *before* the throw so tests can
 *    verify the engine attempted delivery.
 */

import type { GameMoveRequest } from '../../api/client'
import type { GameSnapshot } from '../../api/types'

export class FakeTransport {
  /** Snapshot handed back by `getGame`. Reassign to advance the server view. */
  snapshot: GameSnapshot

  /** Recorded `getGame(gameId)` arguments, in call order. */
  readonly getGameCalls: number[] = []
  /** Recorded `moveGame(gameId, move)` arguments, in call order. */
  readonly moveGameCalls: Array<{ gameId: number; move: GameMoveRequest }> = []
  /** Recorded `nextHand(gameId)` arguments, in call order. */
  readonly nextHandCalls: number[] = []

  /** When set, `moveGame` throws this instead of succeeding. */
  private moveError: unknown = null

  constructor(snapshot: GameSnapshot) {
    this.snapshot = snapshot
  }

  /** Total moveGame invocations recorded (including ones that threw). */
  get moveCount(): number {
    return this.moveGameCalls.length
  }

  /** Cause `moveGame` to reject with `err` until cleared. */
  failMoveWith(err: unknown): void {
    this.moveError = err
  }

  /** Stop failing `moveGame`; subsequent calls resolve normally. */
  clearMoveFailure(): void {
    this.moveError = null
  }

  getGame = async (gameId: number): Promise<GameSnapshot> => {
    this.getGameCalls.push(gameId)
    return this.snapshot
  }

  moveGame = async (gameId: number, move: GameMoveRequest): Promise<{} | null> => {
    // Record before (possibly) throwing so a failed attempt is still observable.
    this.moveGameCalls.push({ gameId, move })
    if (this.moveError !== null) throw this.moveError
    return {}
  }

  nextHand = async (gameId: number): Promise<void> => {
    this.nextHandCalls.push(gameId)
  }
}
