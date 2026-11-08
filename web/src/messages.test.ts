import { describe, expect, it } from 'vitest'
import { ApiError } from './api/client'
import { errorText, movedText, productErrorText } from './messages'

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

describe('errorText new codes', () => {
  it.each(['in_use', 'conflict', 'last_admin'])('has a Polish text for %s', (code) => {
    const text = errorText(new ApiError(409, code, 'English server text'))
    expect(text).not.toBe('English server text')
    expect(text.length).toBeGreaterThan(0)
  })
  it('explains in_use', () => {
    expect(errorText(new ApiError(409, 'in_use', 'x'))).toBe('Ten przepis jest użyty w planie. Najpierw usuń go z planu.')
  })
})

describe('productErrorText', () => {
  it('uses product wording for in_use and conflict', () => {
    expect(productErrorText(new ApiError(409, 'in_use', 'x'))).toBe('Ten produkt jest używany w dzienniku posiłków domowników, więc nie można go usunąć.')
    expect(productErrorText(new ApiError(409, 'conflict', 'x'))).toContain('Produkt o takiej nazwie')
  })
  it('falls back to the generic texts', () => {
    expect(productErrorText(new ApiError(422, 'invalid_input', 'x'))).toBe(errorText(new ApiError(422, 'invalid_input', 'x')))
  })
})

describe('movedText', () => {
  it('uses Polish plural forms', () => {
    expect(movedText(1)).toBe('Przeniesiono 1 pozycję')
    expect(movedText(2)).toBe('Przeniesiono 2 pozycje')
    expect(movedText(5)).toBe('Przeniesiono 5 pozycji')
    expect(movedText(12)).toBe('Przeniesiono 12 pozycji')
    expect(movedText(22)).toBe('Przeniesiono 22 pozycje')
  })
})
