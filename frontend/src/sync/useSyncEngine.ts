/**
 * React binding for the {@link SyncEngine}.
 *
 * `useSyncEngine` is the single hook game screens use instead of the old
 * hand-rolled `useEffect` + `submitMove` dance in `GamePage`. It owns the engine
 * lifecycle (create on mount, start, subscribe, stop on unmount) and exposes the
 * optimistic state plus a `dispatch` callback.
 *
 * Usage:
 * ```tsx
 * const { snapshot, state, connected, dispatch, pendingCount } = useSyncEngine(gameId, user.id)
 * // render `state` (optimistic) instead of a raw server snapshot
 * dispatch({ kind: 'play_card', card: 'JH' })
 * ```
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { SyncEngine, type EngineDeps } from './engine'
import type { EngineGameState, SyncAction } from './types'

export type UseSyncEngineResult = {
  /** The last authoritative snapshot (may be null before first load). */
  snapshot: EngineGameState['confirmedSnapshot']
  /** The optimistic state the UI should render. */
  state: EngineGameState['optimisticState']
  /** Whether the transport is connected. */
  connected: boolean
  /** True while a reconciliation pass is running. */
  reconciling: boolean
  /** Count of actions not yet confirmed by the server. */
  pendingCount: number
  /** Dispatch an optimistic action; returns the generated clientId. */
  dispatch: (action: SyncAction) => string
  /** The most recent reconciliation result (for diagnostics/telemetry). */
  lastReconcile: EngineGameState['lastReconcile']
}

/**
 * Bind a {@link SyncEngine} to component state.
 *
 * @param gameId    the game to sync. Non-positive values disable the engine
 *                  (returns an inert result), matching `GamePage`'s invalid-id
 *                  handling.
 * @param userId    the current user's id, used to resolve "my" position.
 * @param depsOverride optional dependency injection for tests.
 */
export function useSyncEngine(
  gameId: number,
  userId: number | undefined,
  depsOverride?: Partial<EngineDeps>,
): UseSyncEngineResult {
  const enabled = Number.isFinite(gameId) && gameId > 0 && typeof userId === 'number'
  const engineRef = useRef<SyncEngine | null>(null)
  const [engineState, setEngineState] = useState<EngineGameState | null>(null)

  // Freeze the deps override for the lifetime of a given game/user so we don't
  // recreate the engine on every render.
  const deps = useMemo(() => depsOverride, [depsOverride])

  useEffect(() => {
    if (!enabled || typeof userId !== 'number') {
      engineRef.current = null
      setEngineState(null)
      return
    }
    const engine = new SyncEngine(gameId, userId, deps)
    engineRef.current = engine
    const off = engine.subscribe(setEngineState)
    void engine.start()
    return () => {
      off()
      engine.stop()
      engineRef.current = null
    }
  }, [enabled, gameId, userId, deps])

  const dispatch = useCallback((action: SyncAction): string => {
    const engine = engineRef.current
    if (!engine) return ''
    return engine.dispatch(action)
  }, [])

  const pendingCount = useMemo(() => {
    const q = engineState?.queue ?? []
    return q.filter((a) => a.status === 'pending' || a.status === 'inflight').length
  }, [engineState])

  return {
    snapshot: engineState?.confirmedSnapshot ?? null,
    state: engineState?.optimisticState ?? null,
    connected: engineState?.connected ?? false,
    reconciling: engineState?.reconciling ?? false,
    pendingCount,
    dispatch,
    lastReconcile: engineState?.lastReconcile,
  }
}
