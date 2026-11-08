import { describe, expect, it } from 'vitest'
import { formatStock, isExpired, reviewDue, stepFor } from './pantry'
import type { PantryItem } from './api/types'

const item = (o: Partial<PantryItem>): PantryItem => ({
  id: '1', productId: 'p', name: 'Ryż', quantity: 500, unit: 'g', expiresOn: '', staple: false, ...o,
})
const now = new Date(2026, 10, 8, 12)
const daysAgo = (n: number) => new Date(now.getTime() - n * 86400000).toISOString()

describe('reviewDue', () => {
  it('is false without items', () => expect(reviewDue(null, 0, now)).toBe(false))
  it('is true when never reviewed', () => expect(reviewDue(null, 3, now)).toBe(true))
  it('is false after 29 days', () => expect(reviewDue(daysAgo(29), 3, now)).toBe(false))
  it('is true after 31 days', () => expect(reviewDue(daysAgo(31), 3, now)).toBe(true))
})

describe('stepFor', () => {
  it('uses 50 for g and ml, 1 otherwise', () => {
    expect(stepFor('g')).toBe(50)
    expect(stepFor('ml')).toBe(50)
    expect(stepFor('szt')).toBe(1)
    expect(stepFor('puszka')).toBe(1)
  })
})

describe('formatStock', () => {
  it('formats plain and large amounts', () => {
    expect(formatStock(item({}))).toBe('500 g')
    expect(formatStock(item({ quantity: 1500 }))).toBe('1,5 kg')
    expect(formatStock(item({ quantity: 2000, unit: 'ml' }))).toBe('2 l')
    expect(formatStock(item({ quantity: 3, unit: 'szt' }))).toBe('3 szt')
  })
  it('shows staples', () => {
    expect(formatStock(item({ quantity: null, staple: true }))).toBe('stały zapas')
  })
})

describe('isExpired', () => {
  it('compares with the local date', () => {
    expect(isExpired('2026-11-07', now)).toBe(true)
    expect(isExpired('2026-11-08', now)).toBe(false)
    expect(isExpired('', now)).toBe(false)
  })
})
