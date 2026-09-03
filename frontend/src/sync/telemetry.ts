/**
 * Lightweight telemetry sink for the sync engine.
 *
 * The engine emits structured events at meaningful moments (dispatch, flush,
 * reconcile, rollback) so a host app can wire them to logging/metrics without
 * the engine depending on any particular observability stack. The default sink
 * is a no-op; tests inject a `RecordingTelemetry` to assert on emitted events.
 */

import type { ReconcileResult, SyncAction } from './types'

/** The set of telemetry events the engine can emit. */
export type TelemetryEvent =
  | { type: 'dispatch'; gameId: number; clientId: string; action: SyncAction }
  | { type: 'flush_start'; gameId: number; outstanding: number }
  | { type: 'flush_ok'; gameId: number; clientId: string }
  | { type: 'flush_retry'; gameId: number; clientId: string; attempts: number; delayMs: number }
  | { type: 'flush_rejected'; gameId: number; clientId: string; reason: string }
  | { type: 'reconcile'; gameId: number; result: ReconcileResult }
  | { type: 'connected'; gameId: number }
  | { type: 'disconnected'; gameId: number }

/** A telemetry sink receives every emitted event. */
export interface Telemetry {
  emit(event: TelemetryEvent): void
}

/** The default sink: discards everything. */
export const noopTelemetry: Telemetry = {
  emit() {
    /* no-op */
  },
}

/** A sink that records events in memory for assertions in tests. */
export class RecordingTelemetry implements Telemetry {
  readonly events: TelemetryEvent[] = []
  emit(event: TelemetryEvent): void {
    this.events.push(event)
  }
  /** All recorded events of a given type. */
  ofType<T extends TelemetryEvent['type']>(type: T): Extract<TelemetryEvent, { type: T }>[] {
    return this.events.filter((e) => e.type === type) as Extract<TelemetryEvent, { type: T }>[]
  }
  /** Reset the recorded event log. */
  clear(): void {
    this.events.length = 0
  }
}

/** A sink that forwards to `console.debug`, useful during local development. */
export class ConsoleTelemetry implements Telemetry {
  emit(event: TelemetryEvent): void {
    // eslint-disable-next-line no-console
    console.debug('[sync]', event.type, event)
  }
}
