import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { emptyProduct, useProducts } from './products'

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
