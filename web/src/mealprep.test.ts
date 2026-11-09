import { describe, expect, it } from 'vitest'
import { allocationLabel, defaultWeek, prepSources, sessionTitle, staleSessions } from './mealprep'
import type { FreezerItem, PrepSession } from './api/types'

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

describe('entryChip and entryTitle', () => {
  it('labels prep, freezer and out entries', async () => {
    const { entryChip, entryTitle } = await import('./mealprep')
    expect(entryChip({ kind: 'prep', prepDishId: 'd', freezerItemId: null })).toBe('z meal prepu')
    expect(entryChip({ kind: 'prep', prepDishId: null, freezerItemId: 'f' })).toBe('z zamrażalki')
    expect(entryChip({ kind: 'out', prepDishId: null, freezerItemId: null })).toBe('na mieście')
    expect(entryChip({ kind: 'meal', prepDishId: null, freezerItemId: null })).toBe('')
    expect(entryChip({ prepDishId: null, freezerItemId: null })).toBe('')
    expect(entryTitle({ kind: 'out', freeText: '' }, null)).toBe('Na mieście')
    expect(entryTitle({ kind: 'out', freeText: 'Pizzeria' }, null)).toBe('Pizzeria')
    expect(entryTitle({ kind: 'meal', freeText: 'Zupa' }, null)).toBe('Zupa')
    expect(entryTitle({ kind: 'prep', freeText: '' }, 'Curry')).toBe('Curry')
  })
})

describe('prepSources', () => {
  const fz = (id: string, remaining: number): FreezerItem => ({
    id, dishId: null, recipeId: null, title: 'Zupa', portions: 4, allocated: 4 - remaining, remaining, frozenOn: '2026-11-08', allocations: [],
  })
  const own = { kind: 'prep' as const, servings: 2, freeText: 'Zupa', prepDishId: null, freezerItemId: 'f1' }

  it('adds a synthetic option for a fully planned freezer item being edited', () => {
    const list = prepSources([], [], own)
    expect(list).toEqual([{ value: 'f:f1', title: '❄ Zupa · zostało 2', remaining: 2 }])
  })

  it('adds one for a dish with nothing left, titled by the recipe', () => {
    const list = prepSources([], [], { ...own, freezerItemId: null, prepDishId: 'd1' }, 'Gulasz')
    expect(list).toEqual([{ value: 'd:d1', title: 'Gulasz · zostało 2', remaining: 2 }])
  })

  it('adds own servings to a listed source and does not duplicate it', () => {
    const list = prepSources([], [fz('f1', 1)], own)
    expect(list).toHaveLength(1)
    expect(list[0].remaining).toBe(3)
  })

  it('adds nothing when creating or editing a plain meal', () => {
    expect(prepSources([], [], null)).toEqual([])
    expect(prepSources([], [], { ...own, kind: 'meal' })).toEqual([])
  })
})
