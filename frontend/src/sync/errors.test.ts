import { describe, it, expect } from 'vitest'
import { isPermanentError, isTransientError, errorMessage, TRANSIENT_4XX } from './errors'
import { ApiError } from '../lib/http'

describe('isPermanentError', () => {
  it('treats most 4xx as permanent', () => {
    expect(isPermanentError(new ApiError('bad request', 400))).toBe(true)
    expect(isPermanentError(new ApiError('forbidden', 403))).toBe(true)
    expect(isPermanentError(new ApiError('conflict', 409))).toBe(true)
  })

  it('treats transient 4xx as not permanent', () => {
    for (const status of TRANSIENT_4XX) {
      expect(isPermanentError(new ApiError('x', status))).toBe(false)
    }
  })

  it('treats 5xx and network errors as not permanent', () => {
    expect(isPermanentError(new ApiError('boom', 500))).toBe(false)
    expect(isPermanentError(new Error('network'))).toBe(false)
  })
})

describe('isTransientError', () => {
  it('is true for 5xx, transient-4xx, and network errors', () => {
    expect(isTransientError(new ApiError('boom', 503))).toBe(true)
    expect(isTransientError(new ApiError('rate', 429))).toBe(true)
    expect(isTransientError(new Error('offline'))).toBe(true)
  })

  it('is false for permanent 4xx', () => {
    expect(isTransientError(new ApiError('bad', 400))).toBe(false)
  })

  it('is the complement of isPermanentError for ApiError inputs', () => {
    for (const status of [400, 403, 408, 409, 429, 500, 503]) {
      const e = new ApiError('x', status)
      expect(isTransientError(e)).toBe(!isPermanentError(e))
    }
  })
})

describe('errorMessage', () => {
  it('extracts messages from every shape', () => {
    expect(errorMessage(new ApiError('api boom', 500))).toBe('api boom')
    expect(errorMessage(new Error('plain'))).toBe('plain')
    expect(errorMessage('stringy')).toBe('stringy')
    expect(errorMessage(undefined)).toBe('unknown error')
    expect(errorMessage({ weird: true })).toBe('unknown error')
  })
})
