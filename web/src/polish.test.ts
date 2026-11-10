import { describe, expect, it } from 'vitest'
import { portions, shortDate, unitWord } from './polish'
import { formatQuantity } from './stores/recipes'

describe('Polish counts', () => {
  it('agrees portions with the number', () => {
    expect([1, 2, 4, 5, 12, 14, 22, 25, 1.5].map(portions)).toEqual([
      '1 porcja', '2 porcje', '4 porcje', '5 porcji', '12 porcji', '14 porcji', '22 porcje', '25 porcji', '1,5 porcji',
    ])
  })

  it('declines kitchen units and leaves metric ones alone', () => {
    expect(unitWord(1, 'łyżka')).toBe('łyżka')
    expect(unitWord(3, 'łyżka')).toBe('łyżki')
    expect(unitWord(5, 'łyżka')).toBe('łyżek')
    expect(unitWord(0.5, 'szklanka')).toBe('szklanki')
    expect(unitWord(2, 'ząbek')).toBe('ząbki')
    expect(unitWord(7, 'g')).toBe('g')
    expect(formatQuantity(5, 'łyżka')).toBe('5 łyżek')
    expect(formatQuantity(1.5, 'kg')).toBe('1,5 kg')
  })
})

describe('shortDate', () => {
  it('shows day and month, and the year only when it is not this year', () => {
    const now = new Date(2026, 10, 10)
    expect(shortDate('2026-11-12', now)).toBe('12 lis')
    expect(shortDate('2027-01-05', now)).toBe('5 sty 2027')
    expect(shortDate('', now)).toBe('')
  })
})
