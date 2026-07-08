import { describe, it, expect } from 'vitest'
import { applyAction, foldActions, cloneState, cardToCode, cardValue15 } from './reducer'
import type { SyncAction } from './types'
import { card, hand, peggingState, discardState, countingState } from './__fixtures__/state'

describe('cardValue15', () => {
  it('values ace as 1 and face cards as 10', () => {
    expect(cardValue15(card('AH'))).toBe(1)
    expect(cardValue15(card('5S'))).toBe(5)
    expect(cardValue15(card('10D'))).toBe(10)
    expect(cardValue15(card('JD'))).toBe(10)
    expect(cardValue15(card('KC'))).toBe(10)
  })
})

describe('cardToCode', () => {
  it('round-trips compact codes', () => {
    expect(cardToCode(card('AH'))).toBe('AH')
    expect(cardToCode(card('10S'))).toBe('10S')
    expect(cardToCode(card('KD'))).toBe('KD')
  })
})

describe('cloneState', () => {
  it('produces an independent deep copy', () => {
    const s = peggingState()
    const c = cloneState(s)
    c.hands[0].push(card('9C'))
    c.scores[0] = 999
    expect(s.hands[0]).toHaveLength(2) // original untouched
    expect(s.scores[0]).toBe(12)
  })
})

describe('applyAction — play_card', () => {
  it('removes the card, advances the total and the turn', () => {
    const s = peggingState({ pegging_total: 10, current_index: 0 })
    const res = applyAction(s, { kind: 'play_card', card: '5H' }, 0)
    expect(res.rejected).toBeUndefined()
    expect(res.state.hands[0].map(cardToCode)).toEqual(['6H'])
    expect(res.state.pegging_total).toBe(15)
    expect(res.state.pegging_seq.map(cardToCode)).toEqual(['KC', '5H'])
    expect(res.state.last_play_index).toBe(0)
    expect(res.state.current_index).toBe(1)
  })

  it('rejects a card that is not in hand', () => {
    const s = peggingState()
    const res = applyAction(s, { kind: 'play_card', card: '9C' }, 0)
    expect(res.rejected).toMatch(/not in hand/)
    expect(res.state).toBe(s) // unchanged reference
  })

  it('rejects a play that would exceed 31', () => {
    const s = peggingState({ pegging_total: 28, hands: [hand('5H', '6H'), []] })
    const res = applyAction(s, { kind: 'play_card', card: '5H' }, 0)
    expect(res.rejected).toMatch(/exceed 31/)
  })

  it('rejects when it is not your turn', () => {
    const s = peggingState({ current_index: 1 })
    const res = applyAction(s, { kind: 'play_card', card: '5H' }, 0)
    expect(res.rejected).toMatch(/not your turn/)
  })

  it('does not mutate the input state', () => {
    const s = peggingState()
    const before = JSON.stringify(s)
    applyAction(s, { kind: 'play_card', card: '5H' }, 0)
    expect(JSON.stringify(s)).toBe(before)
  })
})

describe('applyAction — discard', () => {
  it('removes discarded cards and marks completion', () => {
    const s = discardState()
    const res = applyAction(s, { kind: 'discard', cards: ['AH', '2H'] }, 0)
    expect(res.rejected).toBeUndefined()
    expect(res.state.hands[0].map(cardToCode)).toEqual(['3H', '4H', '5H', '6H'])
    expect(res.state.discard_completed[0]).toBe(true)
  })

  it('rejects discarding a card not in hand', () => {
    const s = discardState()
    const res = applyAction(s, { kind: 'discard', cards: ['AH', 'KS'] }, 0)
    expect(res.rejected).toMatch(/not in hand/)
  })

  it('rejects discarding during the wrong stage', () => {
    const s = peggingState()
    const res = applyAction(s, { kind: 'discard', cards: ['5H'] }, 0)
    expect(res.rejected).toMatch(/cannot discard/)
  })
})

describe('applyAction — go', () => {
  it('marks the player passed and advances the turn', () => {
    const s = peggingState({ current_index: 0, pegging_passed: [false, false] })
    const res = applyAction(s, { kind: 'go' }, 0)
    expect(res.rejected).toBeUndefined()
    expect(res.state.pegging_passed[0]).toBe(true)
    expect(res.state.current_index).toBe(1)
    // GO must not touch the pegging total — the server owns the reset.
    expect(res.state.pegging_total).toBe(s.pegging_total)
  })
})

describe('applyAction — ready_next_hand', () => {
  it('toggles readiness for the player during counting', () => {
    const s = countingState({ ready_next_hand: [false, false] })
    const on = applyAction(s, { kind: 'ready_next_hand' }, 0)
    expect(on.state.ready_next_hand?.[0]).toBe(true)
    const off = applyAction(on.state, { kind: 'ready_next_hand' }, 0)
    expect(off.state.ready_next_hand?.[0]).toBe(false)
  })

  it('rejects readiness outside the counting stage', () => {
    const res = applyAction(peggingState(), { kind: 'ready_next_hand' }, 0)
    expect(res.rejected).toMatch(/cannot ready up/)
  })
})

describe('foldActions', () => {
  it('applies a sequence of actions in order', () => {
    const s = peggingState({ pegging_total: 0, hands: [hand('5H', '6H'), []], pegging_seq: [] })
    const actions: Array<{ clientId: string; action: SyncAction }> = [
      { clientId: 'a', action: { kind: 'play_card', card: '5H' } },
      // After playing 5H the turn moves to player 1, so player 0's next play is
      // rejected (not their turn) and skipped — exactly the conservative
      // behavior we want.
      { clientId: 'b', action: { kind: 'play_card', card: '6H' } },
    ]
    const { state, skipped } = foldActions(s, actions, 0)
    expect(state.pegging_seq.map(cardToCode)).toEqual(['5H'])
    expect(skipped.map((x) => x.clientId)).toEqual(['b'])
  })

  it('is a no-op for an empty action list', () => {
    const s = peggingState()
    const { state, skipped } = foldActions(s, [], 0)
    expect(skipped).toEqual([])
    expect(state).toEqual(s)
  })
})
