import { describe, it, expect, vi } from 'vitest'
import { noopTelemetry, RecordingTelemetry, ConsoleTelemetry } from './telemetry'

describe('noopTelemetry', () => {
  it('accepts events without throwing', () => {
    expect(() => noopTelemetry.emit({ type: 'connected', gameId: 1 })).not.toThrow()
  })
})

describe('RecordingTelemetry', () => {
  it('records emitted events in order', () => {
    const t = new RecordingTelemetry()
    t.emit({ type: 'connected', gameId: 1 })
    t.emit({ type: 'disconnected', gameId: 1 })
    expect(t.events.map((e) => e.type)).toEqual(['connected', 'disconnected'])
  })

  it('filters by type', () => {
    const t = new RecordingTelemetry()
    t.emit({ type: 'flush_ok', gameId: 1, clientId: 'a' })
    t.emit({ type: 'flush_ok', gameId: 1, clientId: 'b' })
    t.emit({ type: 'connected', gameId: 1 })
    expect(t.ofType('flush_ok')).toHaveLength(2)
    expect(t.ofType('connected')).toHaveLength(1)
  })

  it('clears the log', () => {
    const t = new RecordingTelemetry()
    t.emit({ type: 'connected', gameId: 1 })
    t.clear()
    expect(t.events).toHaveLength(0)
  })
})

describe('ConsoleTelemetry', () => {
  it('forwards to console.debug', () => {
    const spy = vi.spyOn(console, 'debug').mockImplementation(() => {})
    new ConsoleTelemetry().emit({ type: 'connected', gameId: 7 })
    expect(spy).toHaveBeenCalled()
    spy.mockRestore()
  })
})
