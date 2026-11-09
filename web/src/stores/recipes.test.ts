import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { emptyRecipe, formatQuantity, useRecipes } from './recipes'

describe('formatQuantity', () => {
  it('formats amounts in Polish', () => {
    expect(formatQuantity(1.5, 'kg')).toBe('1,5 kg')
    expect(formatQuantity(2, 'szt')).toBe('2 szt')
    expect(formatQuantity(250, '')).toBe('250')
    expect(formatQuantity(null, 'g')).toBe('')
  })
})

describe('emptyRecipe', () => {
  it('starts with sensible defaults', () => {
    const r = emptyRecipe()
    expect(r.baseServings).toBe(4)
    expect(r.ingredients).toEqual([])
    expect(r.id).toBe('')
  })
})

describe('useRecipes', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  function mockFetch(status: number, body: unknown) {
    const spy = vi.fn(async (_url: string, _init?: RequestInit) => new Response(status === 204 ? null : JSON.stringify(body), { status }))
    vi.stubGlobal('fetch', spy)
    return spy
  }

  it('loads the list', async () => {
    const spy = mockFetch(200, [{ id: 'a', title: 'Zupa' }])
    const s = useRecipes()
    await s.load()
    expect(s.items.map((r) => r.title)).toEqual(['Zupa'])
    expect(s.loaded).toBe(true)
    expect(spy.mock.calls[0][0]).toBe('/api/v1/recipes')
  })

  it('creates with POST and updates with PUT', async () => {
    const s = useRecipes()
    let spy = mockFetch(201, { id: 'n1', title: 'Nowy' })
    const created = await s.save({ ...emptyRecipe(), title: 'Nowy' })
    expect(created.id).toBe('n1')
    expect(spy.mock.calls[0][1]!.method).toBe('POST')
    spy = mockFetch(200, { id: 'n1', title: 'Zmieniony' })
    await s.save({ ...emptyRecipe(), id: 'n1', title: 'Zmieniony' })
    expect(spy.mock.calls[0][0]).toBe('/api/v1/recipes/n1')
    expect(spy.mock.calls[0][1]!.method).toBe('PUT')
  })

  it('always sends nutrition and productId, nulls for empty values, 0 kept, and no import hints', async () => {
    const s = useRecipes()
    const spy = mockFetch(200, { id: 'n1', title: 'X' })
    await s.save({
      ...emptyRecipe(),
      id: 'n1',
      title: 'X',
      nutrition: { kcal: '' as unknown as number, protein: 0, carbs: null, fat: 12.5 },
      ingredients: [
        { name: 'jajka', quantity: 2, unit: 'szt', productId: 'p1', raw: '2 jajka', confidence: 'ok' },
        { name: 'sól', quantity: null, unit: '', productId: '' as unknown as string },
        { name: 'pieprz', quantity: null, unit: '' },
      ],
    })
    const body = JSON.parse(spy.mock.calls[0][1]!.body as string)
    expect(body.nutrition).toEqual({ kcal: null, protein: 0, carbs: null, fat: 12.5 })
    expect(body.ingredients.map((i: { productId: unknown }) => i.productId)).toEqual(['p1', null, null])
    expect(body.ingredients[0]).not.toHaveProperty('raw')
    expect(body.ingredients[0]).not.toHaveProperty('confidence')
  })

  it('sends all-null nutrition when the recipe has none', async () => {
    const s = useRecipes()
    const spy = mockFetch(201, { id: 'n1', title: 'X' })
    await s.save({ ...emptyRecipe(), title: 'X' })
    expect(JSON.parse(spy.mock.calls[0][1]!.body as string).nutrition).toEqual({ kcal: null, protein: null, carbs: null, fat: null })
  })

  it('removes from the local list after DELETE', async () => {
    mockFetch(200, [{ id: 'a', title: 'Zupa' }])
    const s = useRecipes()
    await s.load()
    mockFetch(204, null)
    await s.remove('a')
    expect(s.items).toEqual([])
  })

  it('keeps the recipe when DELETE fails', async () => {
    mockFetch(200, [{ id: 'a', title: 'Zupa' }])
    const s = useRecipes()
    await s.load()
    mockFetch(409, { code: 'in_use', message: 'x' })
    await expect(s.remove('a')).rejects.toThrow()
    expect(s.items).toHaveLength(1)
  })

  it('drops completely empty ingredient rows and blank steps', async () => {
    const s = useRecipes()
    const spy = mockFetch(201, { id: 'n1', title: 'X' })
    await s.save({
      ...emptyRecipe(),
      title: 'X',
      ingredients: [
        { name: '', quantity: '' as unknown as number, unit: ' ', note: '', productId: null },
        { name: 'sól', quantity: null, unit: '' },
        { name: '', quantity: 2, unit: 'szt' },
      ],
      steps: ['Gotuj', '  ', ''],
    })
    const body = JSON.parse(spy.mock.calls[0][1]!.body as string)
    expect(body.ingredients.map((i: { name: string }) => i.name)).toEqual(['sól', ''])
    expect(body.steps).toEqual(['Gotuj'])
  })

  it('sends an emptied quantity as null', async () => {
    const s = useRecipes()
    const spy = mockFetch(201, { id: 'n1', title: 'X' })
    await s.save({
      ...emptyRecipe(),
      title: 'X',
      ingredients: [{ name: 'sól', quantity: '' as unknown as number, unit: '' }],
    })
    const body = JSON.parse(spy.mock.calls[0][1]!.body as string)
    expect(body.ingredients[0].quantity).toBeNull()
  })

  it('sends only schema fields when saving an imported recipe', async () => {
    const spy = mockFetch(201, { id: 'n' })
    const s = useRecipes()
    await s.save({
      id: '', title: 'Zupa', baseServings: 4, sourceUrl: '', tags: [], steps: [],
      ingredients: [{ name: 'woda', quantity: 1, unit: 'l', raw: '1 l wody', note: 'zimna', confidence: 'guess' }],
    })
    const body = JSON.parse(String(spy.mock.calls[0][1]?.body))
    expect(body.ingredients[0]).toEqual({ name: 'woda', quantity: 1, unit: 'l', note: 'zimna', productId: null })
  })

  it('sends the ingredient note: kept, defaulted to empty, trimmed, never raw/confidence', async () => {
    const spy = mockFetch(201, { id: 'n' })
    const s = useRecipes()
    await s.save({
      id: '', title: 'Zupa', baseServings: 4, sourceUrl: '', tags: [], steps: [],
      ingredients: [
        { name: 'a', quantity: 1, unit: '', note: 'drobno starta' },
        { name: 'b', quantity: 1, unit: '' },
        { name: 'c', quantity: 1, unit: '', note: '  spacje \n', raw: 'x', confidence: 'ok' },
      ],
    })
    const ing = JSON.parse(String(spy.mock.calls[0][1]?.body)).ingredients
    expect(ing[0].note).toBe('drobno starta')
    expect(ing[1].note).toBe('')
    expect(ing[2].note).toBe('spacje')
    for (const i of ing) {
      expect(i).not.toHaveProperty('raw')
      expect(i).not.toHaveProperty('confidence')
    }
  })
})
