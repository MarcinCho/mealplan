import { describe, expect, it } from 'vitest'
import type { PlanEntry } from './api/types'
import { avatarColor, entriesOfDay, freeSlots, greeting, heroOf, initial, suggest } from './today'

const e = (id: string, slot: PlanEntry['slot'], done = false, date = '2026-11-03'): PlanEntry => ({
  id, date, slot, recipeId: null, freeText: id, servings: 2, cookUserId: null, done,
})

describe('entriesOfDay', () => {
  it('keeps only that day, in meal order', () => {
    const got = entriesOfDay([e('d', 'dinner'), e('x', 'lunch', false, '2026-11-04'), e('b', 'breakfast')], '2026-11-03')
    expect(got.map((x) => x.id)).toEqual(['b', 'd'])
  })
})

describe('heroOf', () => {
  it('is empty when nothing is planned', () => {
    expect(heroOf([e('x', 'lunch', false, '2026-11-04')], '2026-11-03')).toEqual({ kind: 'empty' })
  })
  it('picks the first meal that is not done yet', () => {
    const h = heroOf([e('b', 'breakfast', true), e('l', 'lunch'), e('d', 'dinner')], '2026-11-03')
    expect(h.kind === 'next' && h.entry.id).toBe('l')
  })
  it('reports finished when every meal is done', () => {
    expect(heroOf([e('b', 'breakfast', true), e('l', 'lunch', true)], '2026-11-03')).toEqual({ kind: 'finished' })
  })
})

describe('freeSlots', () => {
  it('offers the main meals that are still free, never snacks', () => {
    expect(freeSlots([e('l', 'lunch')], '2026-11-03')).toEqual(['breakfast', 'dinner'])
    expect(freeSlots([], '2026-11-03')).toEqual(['breakfast', 'lunch', 'dinner'])
  })
})

describe('people', () => {
  it('gives each member a stable color and wraps around', () => {
    const ids = ['a', 'b']
    expect(avatarColor('a', ids)).not.toBe(avatarColor('b', ids))
    expect(avatarColor('a', ids)).toBe(avatarColor('a', ids))
    expect(avatarColor('missing', ids)).toBe(avatarColor('a', ids))
  })
  it('takes the first letter, upper-cased', () => {
    expect(initial('  ania')).toBe('A')
    expect(initial('łukasz')).toBe('Ł')
    expect(initial('')).toBe('?')
  })
})

describe('greeting', () => {
  it('follows the hour', () => {
    expect([2, 9, 20].map(greeting)).toEqual(['Dobrej nocy', 'Dzień dobry', 'Dobry wieczór'])
  })
})

describe('suggest', () => {
  const items = [{ title: 'Zupa pomidorowa' }, { title: 'Pomidory z mozzarellą' }, { title: 'Makaron' }]
  it('is empty for a blank query', () => expect(suggest(items, '  ')).toEqual([]))
  it('matches anywhere, case-insensitively, prefix first', () => {
    expect(suggest(items, 'POMI').map((i) => i.title)).toEqual(['Pomidory z mozzarellą', 'Zupa pomidorowa'])
  })
  it('respects the limit', () => expect(suggest(items, 'o', 1)).toHaveLength(1))
})
