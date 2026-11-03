import { describe, expect, it } from 'vitest'
import { draftToRecipe, findDuplicate, normalizeUrl } from './importDraft'
import type { Recipe, RecipeDraft } from './api/types'

function draft(over: Partial<RecipeDraft> = {}): RecipeDraft {
  return {
    title: 'Zupa',
    servings: 4,
    sourceUrl: 'https://Example.com/zupa/',
    tags: ['zupa'],
    ingredients: [
      { raw: '1 l wody', name: 'wody', quantity: 1, unit: 'l', note: '', confidence: 'ok' },
      { raw: '???', name: '', quantity: null, unit: '', note: '', confidence: 'unparsed' },
    ],
    steps: ['Gotuj.'],
    warnings: [],
    ...over,
  }
}

function recipe(over: Partial<Recipe>): Recipe {
  return { id: 'r1', title: 'Inna', baseServings: 4, sourceUrl: '', tags: [], ingredients: [], steps: [], ...over }
}

describe('draftToRecipe', () => {
  it('maps a draft and keeps per-row import hints', () => {
    const r = draftToRecipe(draft())
    expect(r.id).toBe('')
    expect(r.baseServings).toBe(4)
    expect(r.ingredients[0]).toMatchObject({ name: 'wody', quantity: 1, unit: 'l', confidence: 'ok' })
  })
  it('defaults unknown servings to 4 and keeps the id when updating', () => {
    const r = draftToRecipe(draft({ servings: 0 }), 'abc')
    expect(r.baseServings).toBe(4)
    expect(r.id).toBe('abc')
  })
  it('never loses an unparsed line: the raw text becomes the name, cut to 200 characters', () => {
    const r = draftToRecipe(draft({ ingredients: [{ raw: 'x'.repeat(300), name: '', quantity: null, unit: '', note: '', confidence: 'unparsed' }] }))
    expect(r.ingredients[0].name).toHaveLength(200)
  })
  it('copies arrays so editing the recipe does not change the draft', () => {
    const d = draft()
    const r = draftToRecipe(d)
    r.steps.push('x')
    r.tags.push('y')
    expect(d.steps).toHaveLength(1)
    expect(d.tags).toHaveLength(1)
  })
})

describe('normalizeUrl', () => {
  it('ignores scheme, www, case, fragment and trailing slashes', () => {
    expect(normalizeUrl('HTTPS://www.Example.com/Zupa/#kroki')).toBe('example.com/zupa')
    expect(normalizeUrl('http://example.com/zupa')).toBe('example.com/zupa')
    expect(normalizeUrl('')).toBe('')
  })
})

describe('findDuplicate', () => {
  it('matches by source address or by title, ignoring case', () => {
    const byUrl = recipe({ id: 'a', sourceUrl: 'http://www.example.com/zupa' })
    const byTitle = recipe({ id: 'b', title: ' zupa ' })
    expect(findDuplicate(draft(), [recipe({ id: 'x' }), byUrl])?.id).toBe('a')
    expect(findDuplicate(draft({ sourceUrl: '' }), [recipe({ id: 'x' }), byTitle])?.id).toBe('b')
  })
  it('does not match unrelated recipes or empty values', () => {
    expect(findDuplicate(draft({ sourceUrl: '', title: '' }), [recipe({ title: '', sourceUrl: '' })])).toBeUndefined()
    expect(findDuplicate(draft(), [recipe({})])).toBeUndefined()
  })
})
