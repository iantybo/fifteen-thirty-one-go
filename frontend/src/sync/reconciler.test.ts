import { describe, it, expect } from 'vitest'
import { planReconcile, dropReconciled, type ReconcileInput } from './reconciler'

/** A baseline input the individual tests tweak. */
function input(overrides: Partial<ReconcileInput> = {}): ReconcileInput {
  return {
    currentRevision: 5,
    incomingRevision: 6,
    hasConfirmed: true,
    outstanding: ['a', 'b', 'c'],
    accepted: [],
    rejected: [],
    ...overrides,
  }
}

describe('planReconcile', () => {
  it('ignores a stale snapshot once we have confirmed one', () => {
    const res = planReconcile(input({ currentRevision: 5, incomingRevision: 5, hasConfirmed: true }))
    expect(res.ignoredStale).toBe(true)
    expect(res.confirmed).toEqual([])
    expect(res.rejected).toEqual([])
    expect(res.keptPending).toEqual(['a', 'b', 'c'])
    expect(res.revision).toBe(5)
  })

  it('ignores a snapshot whose revision moves backward', () => {
    const res = planReconcile(input({ currentRevision: 7, incomingRevision: 3, hasConfirmed: true }))
    expect(res.ignoredStale).toBe(true)
    expect(res.revision).toBe(7)
  })

  it('does NOT ignore the very first snapshot even when the revision does not advance', () => {
    const res = planReconcile(
      input({ currentRevision: 0, incomingRevision: 0, hasConfirmed: false, outstanding: [] }),
    )
    expect(res.ignoredStale).toBe(false)
    expect(res.revision).toBe(0)
  })

  it('confirms the accepted ids on advance', () => {
    const res = planReconcile(input({ accepted: ['a'], outstanding: ['a', 'b', 'c'] }))
    expect(res.ignoredStale).toBe(false)
    expect(res.confirmed).toEqual(['a'])
  })

  it('reports the rejected ids on advance', () => {
    const res = planReconcile(input({ rejected: ['b'] }))
    expect(res.rejected).toEqual(['b'])
  })

  it('drops confirmed and rejected from keptPending, keeping the rest in order', () => {
    const res = planReconcile(input({ outstanding: ['a', 'b', 'c', 'd'], accepted: ['a'], rejected: ['c'] }))
    expect(res.keptPending).toEqual(['b', 'd'])
  })

  it('advances the revision to the incoming value', () => {
    const res = planReconcile(input({ currentRevision: 5, incomingRevision: 9 }))
    expect(res.revision).toBe(9)
  })

  it('does not mutate the input arrays', () => {
    const outstanding = ['a', 'b']
    const accepted = ['a']
    const rejected: string[] = []
    planReconcile({
      currentRevision: 1,
      incomingRevision: 2,
      hasConfirmed: true,
      outstanding,
      accepted,
      rejected,
    })
    expect(outstanding).toEqual(['a', 'b'])
    expect(accepted).toEqual(['a'])
    expect(rejected).toEqual([])
  })
})

describe('dropReconciled', () => {
  it('removes accepted and rejected ids while preserving order', () => {
    expect(dropReconciled(['a', 'b', 'c', 'd'], ['b'], ['d'])).toEqual(['a', 'c'])
  })

  it('returns a fresh array and never mutates the input', () => {
    const outstanding = ['a', 'b']
    const out = dropReconciled(outstanding, [], [])
    expect(out).toEqual(['a', 'b'])
    expect(out).not.toBe(outstanding)
  })
})
