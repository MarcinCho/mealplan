import { describe, expect, it } from 'vitest'
import { allocationLabel, defaultWeek, sessionTitle, staleSessions } from './mealprep'
import type { PrepSession } from './api/types'

const sess = (date: string, remaining: number): PrepSession => ({
  id: date, date, name: 'x', week: '2026-11-02', cooked: false, remaining, dishes: [],
})

describe('defaultWeek', () => {
  it.each([
    ['2026-11-08', '2026-11-09'],
    ['2026-11-07', '2026-11-09'],
    ['2026-11-04', '2026-11-02'],
    ['2026-11-09', '2026-11-09'],
    ['2026-11-13', '2026-11-09'],
  ])('%s -> %s', (d, w) => expect(defaultWeek(d)).toBe(w))
})

describe('sessionTitle', () => {
  it('uses day and roman month', () => {
    expect(sessionTitle('2026-11-02')).toBe('Gotowanie 2 XI')
    expect(sessionTitle('2026-12-24')).toBe('Gotowanie 24 XII')
    expect(sessionTitle('2026-01-05')).toBe('Gotowanie 5 I')
  })
})

describe('staleSessions', () => {
  it('flags sessions older than 4 days with portions left', () => {
    const list = [sess('2026-11-08', 2), sess('2026-11-08', 0), sess('2026-11-09', 3)]
    expect(staleSessions(list, '2026-11-13')).toEqual([list[0]])
  })
})

describe('allocationLabel', () => {
  it('formats day, slot and servings', () => {
    expect(allocationLabel({ date: '2026-11-09', slot: 'breakfast', servings: 2 })).toBe('Pn śniadanie ×2')
    expect(allocationLabel({ date: '2026-11-15', slot: 'dinner', servings: 1 })).toBe('Nd kolacja ×1')
  })
})
