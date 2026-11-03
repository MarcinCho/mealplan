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
    expect(body.ingredients[0]).toEqual({ name: 'woda', quantity: 1, unit: 'l' })
  })
})
