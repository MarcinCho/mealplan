import { describe, expect, it } from 'vitest'
import { draftToRecipe, findDuplicate, importErrorText, keepsDraft, mergeIntoExisting, normalizeImportUrl, normalizeUrl } from './importDraft'
import { ApiError } from './api/client'
import { t } from './messages'
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

describe('keepsDraft', () => {
  it('keeps the draft only when heading to the recipe editor', () => {
    expect(keepsDraft('recipe-new')).toBe(true)
    expect(keepsDraft('recipe-edit')).toBe(true)
    for (const n of ['recipes', 'home', 'recipe-import', undefined]) expect(keepsDraft(n)).toBe(false)
  })
})

describe('mergeIntoExisting', () => {
  const existing = recipe({
    id: 'r9', title: 'Stary', baseServings: 6, sourceUrl: 'https://old.example/z', tags: ['stare'],
    ingredients: [{ name: 'stary', quantity: 1, unit: 'g' }], steps: ['stary krok'],
  })
  it('replaces ingredients and steps, keeps the id', () => {
    const m = mergeIntoExisting(draft(), existing)
    expect(m.id).toBe('r9')
    expect(m.title).toBe('Zupa')
    expect(m.baseServings).toBe(4)
    expect(m.sourceUrl).toBe('https://Example.com/zupa/')
    expect(m.tags).toEqual(['zupa'])
    expect(m.steps).toEqual(['Gotuj.'])
    expect(m.ingredients[0].name).toBe('wody')
  })
  it('keeps what the draft did not carry', () => {
    const m = mergeIntoExisting(draft({ title: '  ', servings: 0, sourceUrl: '', tags: [] }), existing)
    expect(m.title).toBe('Stary')
    expect(m.baseServings).toBe(6)
    expect(m.sourceUrl).toBe('https://old.example/z')
    expect(m.tags).toEqual(['stare'])
    expect(m.tags).not.toBe(existing.tags)
  })
})

describe('normalizeImportUrl', () => {
  it('adds https when there is no scheme and leaves others alone', () => {
    expect(normalizeImportUrl('  example.com/zupa ')).toBe('https://example.com/zupa')
    expect(normalizeImportUrl('http://example.com/z')).toBe('http://example.com/z')
    expect(normalizeImportUrl('HTTPS://Example.com')).toBe('HTTPS://Example.com')
    expect(normalizeImportUrl('')).toBe('')
  })
})

describe('importErrorText', () => {
  it('shows the Polish server text of invalid_input without the English prefix', () => {
    expect(importErrorText(new ApiError(422, 'invalid_input', 'invalid input: tekst przepisu jest pusty'), 'text')).toBe('tekst przepisu jest pusty')
  })
  it('explains unparsable text differently from unparsable links', () => {
    const e = new ApiError(422, 'import_unparsable', 'x')
    expect(importErrorText(e, 'text')).toBe(t.importTextUnparsable)
    expect(importErrorText(e, 'url')).not.toBe(t.importTextUnparsable)
    expect(importErrorText(e, 'url')).toContain('wkleić')
  })
  it('falls back to the common translations', () => {
    expect(importErrorText(new ApiError(504, 'import_timeout', 'x'), 'url')).toContain('wolno')
    expect(importErrorText(new Error('boom'), 'url')).toBe('Coś poszło nie tak. Spróbuj ponownie.')
  })
})
