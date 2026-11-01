import { describe, expect, it } from 'vitest'
import { mondayOf, shiftWeek, weekDates, SLOTS } from './weeks'

describe('weeks', () => {
  it('finds the monday of any day, including sunday', () => {
    expect(mondayOf(new Date(2026, 10, 4))).toBe('2026-11-02') // Wednesday
    expect(mondayOf(new Date(2026, 10, 2))).toBe('2026-11-02') // Monday
    expect(mondayOf(new Date(2026, 10, 8))).toBe('2026-11-02') // Sunday belongs to the week before
  })

  it('shifts across month and year boundaries', () => {
    expect(shiftWeek('2026-11-02', 1)).toBe('2026-11-09')
    expect(shiftWeek('2026-12-28', 1)).toBe('2027-01-04')
    expect(shiftWeek('2026-01-05', -1)).toBe('2025-12-29')
  })

  it('lists seven dates', () => {
    const d = weekDates('2026-12-28')
    expect(d).toHaveLength(7)
    expect(d[6]).toBe('2027-01-03')
  })

  it('keeps slot order stable', () => {
    expect(SLOTS.map((s) => s.value)).toEqual(['breakfast', 'lunch', 'dinner', 'snack'])
  })
})
