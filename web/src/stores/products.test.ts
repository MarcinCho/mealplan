import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { emptyProduct, normalizeQuery, useProducts } from './products'

describe('useProducts', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  function mockFetch(status: number, body: unknown) {
    const spy = vi.fn(async (_url: string, _init?: RequestInit) => new Response(status === 204 ? null : JSON.stringify(body), { status }))
    vi.stubGlobal('fetch', spy)
    return spy
  }

  it('loads without a query', async () => {
    const spy = mockFetch(200, [{ id: 'a', name: 'Mleko' }])
    const s = useProducts()
    await s.load()
    expect(s.items.map((p) => p.name)).toEqual(['Mleko'])
    expect(spy.mock.calls[0][0]).toBe('/api/v1/products')
  })

  it('encodes the search query', async () => {
    const spy = mockFetch(200, [])
    await useProducts().load('mąka & jajka')
    expect(spy.mock.calls[0][0]).toBe('/api/v1/products?q=' + encodeURIComponent('mąka & jajka'))
  })

  it('creates with POST and only schema fields, emptied numbers as null', async () => {
    const spy = mockFetch(201, { id: 'n' })
    const s = useProducts()
    const p = { ...emptyProduct(), name: ' Ser ', pieceGrams: '' as unknown as number, aliases: ['ser żółty'] }
    p.per100g.kcal = 350
    await s.save(p)
    expect(spy.mock.calls[0][0]).toBe('/api/v1/products')
    expect(spy.mock.calls[0][1]!.method).toBe('POST')
    const body = JSON.parse(String(spy.mock.calls[0][1]!.body))
    expect(body).toEqual({
      name: 'Ser',
      per100g: { kcal: 350, protein: null, carbs: null, fat: null },
      pieceGrams: null,
      densityGPerMl: null,
      aliases: ['ser żółty'],
    })
  })

  it('updates with PUT', async () => {
    const spy = mockFetch(200, { id: 'p 1' })
    await useProducts().save({ ...emptyProduct(), id: 'p 1', name: 'X' })
    expect(spy.mock.calls[0][0]).toBe('/api/v1/products/p%201')
    expect(spy.mock.calls[0][1]!.method).toBe('PUT')
  })

  it('removes from the list after DELETE and keeps it on 409', async () => {
    mockFetch(200, [{ id: 'a', name: 'Mleko' }])
    const s = useProducts()
    await s.load()
    mockFetch(409, { code: 'in_use', message: 'x' })
    await expect(s.remove('a')).rejects.toThrow()
    expect(s.items).toHaveLength(1)
    const spy = mockFetch(204, null)
    await s.remove('a')
    expect(spy.mock.calls[0][1]!.method).toBe('DELETE')
    expect(s.items).toEqual([])
  })
})

describe('normalizeQuery', () => {
  it('treats null, undefined and blank as empty and trims', () => {
    expect(normalizeQuery(null)).toBe('')
    expect(normalizeQuery(undefined)).toBe('')
    expect(normalizeQuery('')).toBe('')
    expect(normalizeQuery('  mleko ')).toBe('mleko')
  })
})

describe('useProducts.load with a cleared search', () => {
  it('reloads the full list when the query is null', async () => {
    setActivePinia(createPinia())
    const spy = vi.fn(async (_u: string) => new Response('[]', { status: 200 }))
    vi.stubGlobal('fetch', spy)
    await useProducts().load(null)
    await useProducts().load('  mleko ')
    expect(spy.mock.calls[0][0]).toBe('/api/v1/products')
    expect(spy.mock.calls[1][0]).toBe('/api/v1/products?q=mleko')
  })

})

describe('useProducts shared catalog', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })
  function mockFetch(status: number, body: unknown) {
    const spy = vi.fn(async (_url: string, _init?: RequestInit) => new Response(status === 204 ? null : JSON.stringify(body), { status }))
    vi.stubGlobal('fetch', spy)
    return spy
  }

  it('fetches the full list once for many callers', async () => {
    const spy = mockFetch(200, [{ id: 'a', name: 'Mleko' }])
    const s = useProducts()
    await Promise.all(Array.from({ length: 25 }, () => s.ensureLoaded()))
    await s.ensureLoaded()
    expect(spy).toHaveBeenCalledTimes(1)
    expect(spy.mock.calls[0][0]).toBe('/api/v1/products')
    expect(s.catalog.map((p) => p.id)).toEqual(['a'])
  })

  it('retries after a failed fetch', async () => {
    const s = useProducts()
    mockFetch(500, { error: { code: 'internal', message: 'x' } })
    await expect(s.ensureLoaded()).rejects.toBeTruthy()
    const spy = mockFetch(200, [{ id: 'a', name: 'Mleko' }])
    await s.ensureLoaded()
    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('resolves an id from the cache without a request', async () => {
    const spy = mockFetch(200, [{ id: 'a', name: 'Mleko' }])
    const s = useProducts()
    expect((await s.resolve('a'))?.name).toBe('Mleko')
    expect((await s.resolve('a'))?.name).toBe('Mleko')
    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('refreshes once for an id missing from the cache, then gives up', async () => {
    const s = useProducts()
    mockFetch(200, [{ id: 'a', name: 'Mleko' }])
    await s.ensureLoaded()
    const spy = mockFetch(200, [{ id: 'a', name: 'Mleko' }, { id: 'b', name: 'Mąka' }])
    expect((await s.resolve('b'))?.name).toBe('Mąka')
    expect(spy).toHaveBeenCalledTimes(1)
    expect(await s.resolve('zzz')).toBeNull()
    expect(await s.resolve(null)).toBeNull()
  })

  it('adds a created product to the catalog', async () => {
    mockFetch(200, [])
    const s = useProducts()
    await s.ensureLoaded()
    s.addToCatalog({ id: 'n', name: 'Nowy' } as never)
    expect(s.catalog.map((p) => p.id)).toEqual(['n'])
  })

  it('save upserts into the loaded catalog (create and rename)', async () => {
    const s = useProducts()
    mockFetch(200, [{ id: 'a', name: 'Mleko' }])
    await s.ensureLoaded()
    mockFetch(201, { id: 'n', name: 'Nowy' })
    await s.save({ ...emptyProduct(), name: 'Nowy' })
    expect(s.catalog.map((p) => p.id)).toEqual(['n', 'a'])
    mockFetch(200, { id: 'a', name: 'Mleko 2%' })
    await s.save({ ...emptyProduct(), id: 'a', name: 'Mleko 2%' })
    expect(s.catalog.find((p) => p.id === 'a')?.name).toBe('Mleko 2%')
    expect(s.catalog).toHaveLength(2)
  })

  it('remove drops the product from the catalog', async () => {
    const s = useProducts()
    mockFetch(200, [{ id: 'a', name: 'Mleko' }, { id: 'b', name: 'Ser' }])
    await s.ensureLoaded()
    mockFetch(204, null)
    await s.remove('a')
    expect(s.catalog.map((p) => p.id)).toEqual(['b'])
  })

  it('ensureLoaded(true) refetches a loaded catalog', async () => {
    const s = useProducts()
    mockFetch(200, [{ id: 'a', name: 'Mleko' }])
    await s.ensureLoaded()
    const spy = mockFetch(200, [{ id: 'b', name: 'Ser' }])
    await s.ensureLoaded(true)
    expect(spy).toHaveBeenCalledTimes(1)
    expect(s.catalog.map((p) => p.id)).toEqual(['b'])
  })
})
