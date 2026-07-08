import { describe, it, expect } from 'vitest'
import { MemoryStore } from './MemoryStore'

describe('MemoryStore', () => {
  it('stores and retrieves values', () => {
    const s = new MemoryStore()
    s.setItem('a', '1')
    expect(s.getItem('a')).toBe('1')
  })

  it('returns null for missing keys', () => {
    expect(new MemoryStore().getItem('nope')).toBeNull()
  })

  it('removes values', () => {
    const s = new MemoryStore()
    s.setItem('a', '1')
    s.removeItem('a')
    expect(s.getItem('a')).toBeNull()
  })

  it('overwrites existing values', () => {
    const s = new MemoryStore()
    s.setItem('a', '1')
    s.setItem('a', '2')
    expect(s.getItem('a')).toBe('2')
    expect(s.size()).toBe(1)
  })
})
