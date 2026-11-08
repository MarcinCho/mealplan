import { describe, expect, it } from 'vitest'
import type { DaySummary, Macros } from './api/types'
import { dayWarnings, formatMacros, formatNumber, formatRemaining, dayLine, goalPercent, validQuantity } from './nutrition'

const m = (kcal: number | null, protein: number | null, carbs: number | null, fat: number | null): Macros => ({ kcal, protein, carbs, fat })
const none = { kcal: false, protein: false, carbs: false, fat: false }

function day(total: Macros, goal: Macros, incomplete = none): DaySummary {
  return {
    date: '2026-11-08', total, goal, remaining: m(null, null, null, null), meals: [], log: [],
    missingMeals: 0, missingLog: 0, incomplete, approximate: false,
  }
}

describe('formatNumber', () => {
  it('rounds kcal to whole numbers and grams to one decimal with a comma', () => {
    expect(formatNumber(639.6, 'kcal')).toBe('640')
    expect(formatNumber(12.34, 'g')).toBe('12,3')
    expect(formatNumber(12, 'g')).toBe('12')
    expect(formatNumber(null, 'g')).toBe('—')
  })
})

describe('formatMacros', () => {
  it('formats all values', () => {
    expect(formatMacros(m(639.6, 38.04, 100.25, 12.5))).toBe('640 kcal · 38 g białka · 100,3 g węglowodanów · 12,5 g tłuszczu')
  })
  it('skips null values', () => {
    expect(formatMacros(m(500, null, null, 7.25))).toBe('500 kcal · 7,3 g tłuszczu')
  })
  it('is a dash when nothing is known', () => {
    expect(formatMacros(m(null, null, null, null))).toBe('—')
  })
})

describe('formatRemaining', () => {
  it('says there is no goal', () => {
    expect(formatRemaining(day(m(100, 1, 1, 1), m(null, null, null, null)))).toBe('brak celu')
  })
  it('reports a precise shortfall when complete', () => {
    expect(formatRemaining(day(m(1360, 62, 0, 0), m(2000, 100, null, null)))).toBe('brakuje 640 kcal · 38 g białka')
  })
  it('says goal reached only when complete and at or above goal', () => {
    expect(formatRemaining(day(m(2000, 120, 0, 0), m(2000, 100, null, null)))).toBe('cel osiągnięty')
    expect(formatRemaining(day(m(2300, 0, 0, 0), m(2000, null, null, null)))).toBe('cel osiągnięty')
  })
  it('does not claim a precise shortfall when the total is incomplete', () => {
    const d = day(m(1360, null, null, null), m(2000, null, null, null), { ...none, kcal: true })
    expect(formatRemaining(d)).toBe('brakuje co najmniej 640 kcal')
  })
  it('does not claim the goal is reached when the total is incomplete', () => {
    const d = day(m(2100, null, null, null), m(2000, null, null, null), { ...none, kcal: true })
    expect(formatRemaining(d)).not.toContain('cel osiągnięty')
    expect(formatRemaining(d)).toBe('dane o kaloriach niepełne')
  })
  it('says there is no data when the total is null', () => {
    expect(formatRemaining(day(m(null, null, null, null), m(2000, null, null, null)))).toBe('brak danych o kaloriach')
  })
  it('combines precise, lower-bound and missing parts', () => {
    const d = day(m(1360, 50, null, null), m(2000, 100, 300, null), { ...none, protein: true })
    expect(formatRemaining(d)).toBe('brakuje 640 kcal · brakuje co najmniej 50 g białka · brak danych o węglowodanach')
  })
})

describe('goalPercent', () => {
  it('is the share of the goal, clamped to 0..100', () => {
    expect(goalPercent(500, 2000)).toBe(25)
    expect(goalPercent(3000, 2000)).toBe(100)
    expect(goalPercent(-5, 2000)).toBe(0)
  })
  it('is 0 without a total and null without a goal', () => {
    expect(goalPercent(null, 2000)).toBe(0)
    expect(goalPercent(100, null)).toBeNull()
    expect(goalPercent(100, 0)).toBeNull()
  })
})

describe('dayWarnings', () => {
  const base = day(m(1, 1, 1, 1), m(1, 1, 1, 1))
  it('is empty for a complete day', () => {
    expect(dayWarnings(base)).toEqual([])
  })
  it('pluralises the missing counts in Polish', () => {
    expect(dayWarnings({ ...base, missingMeals: 1 })).toEqual(['1 posiłek bez wartości'])
    expect(dayWarnings({ ...base, missingMeals: 3, missingLog: 5 })).toEqual(['3 posiłki bez wartości', '5 pozycji bez wartości'])
    expect(dayWarnings({ ...base, missingLog: 2 })).toEqual(['2 pozycje bez wartości'])
    expect(dayWarnings({ ...base, missingMeals: 12 })).toEqual(['12 posiłków bez wartości'])
  })
  it('adds the approximate warning last', () => {
    expect(dayWarnings({ ...base, missingMeals: 2, approximate: true })).toEqual(['2 posiłki bez wartości', 'wartości przybliżone'])
  })
})

describe('validQuantity', () => {
  it('mirrors the API: > 0 and <= 10000', () => {
    expect(validQuantity(0)).toBe(false)
    expect(validQuantity(-1)).toBe(false)
    expect(validQuantity(0.5)).toBe(true)
    expect(validQuantity(10000)).toBe(true)
    expect(validQuantity(10001)).toBe(false)
    expect(validQuantity(null)).toBe(false)
    expect(validQuantity(NaN)).toBe(false)
  })
})

describe('dayLine', () => {
  it('shows kcal against the goal and protein', () => {
    expect(dayLine(day(m(1850, 92.4, 1, 1), m(2000, 100, null, null)))).toBe('1850 / 2000 kcal · białko 92,4 g')
  })
  it('uses a dash for missing values and omits a missing goal', () => {
    expect(dayLine(day(m(null, null, null, null), m(null, null, null, null)))).toBe('— kcal · białko — g')
  })
})

describe('dayLine incompleteness', () => {
  const inc = (o: Partial<Record<keyof Macros, boolean>>) => ({ ...none, ...o })
  it('marks an incomplete known total with a lower bound', () => {
    expect(dayLine(day(m(1850, 92.4, 1, 1), m(2000, 100, null, null), inc({ kcal: true })))).toBe('≥1850 / 2000 kcal · białko 92,4 g')
    expect(dayLine(day(m(1850, 92.4, 1, 1), m(2000, 100, null, null), inc({ protein: true })))).toBe('1850 / 2000 kcal · białko ≥92,4 g')
  })
  it('shows a dash for an unknown total even with a goal', () => {
    expect(dayLine(day(m(null, null, 1, 1), m(2000, 100, null, null), inc({ kcal: true })))).toBe('— / 2000 kcal · białko — g')
  })
})
