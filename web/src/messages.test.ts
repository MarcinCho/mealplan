import { describe, expect, it } from 'vitest'
import { ApiError } from './api/client'
import { errorText } from './messages'

describe('errorText', () => {
  it.each([
    'unauthenticated',
    'csrf',
    'not_found',
    'too_large',
    'bad_request',
    'internal',
    'unsupported_media_type',
  ])('translates %s instead of echoing server text', (code) => {
    const text = errorText(new ApiError(400, code, 'English server text'))
    expect(text).not.toBe('English server text')
    expect(text.length).toBeGreaterThan(0)
  })

  it('uses retryAfter for rate_limited when present', () => {
    expect(errorText(new ApiError(429, 'rate_limited', 'x', 42))).toContain('42 s')
  })

  it('falls back for rate_limited without retryAfter', () => {
    expect(errorText(new ApiError(429, 'rate_limited', 'x'))).toBe('Zbyt wiele prób. Spróbuj ponownie za chwilę.')
  })

  it('falls back to server text for unknown codes and generic text for non-API errors', () => {
    expect(errorText(new ApiError(400, 'weird', 'server says'))).toBe('server says')
    expect(errorText(new Error('boom'))).toBe('Coś poszło nie tak. Spróbuj ponownie.')
  })
})
