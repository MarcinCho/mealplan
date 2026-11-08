import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { usePantry } from './pantry'

const row = (o = {}) => ({ id: 'a', productId: 'p', name: 'Ryż', quantity: 500, unit: 'g', expiresOn: '', staple: false, ...o })

describe('usePantry', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })
  function mockFetch(status: number, body: unknown) {
    const spy = vi.fn(async (_url: string, _init?: RequestInit) => new Response(status === 204 ? null : JSON.stringify(body), { status }))
    vi.stubGlobal('fetch', spy)
    return spy
  }

  it('loads items and reviewedAt', async () => {
    const spy = mockFetch(200, { items: [row()], reviewedAt: '2026-10-01T10:00:00Z' })
    const s = usePantry()
    await s.load()
    expect(spy.mock.calls[0][0]).toBe('/api/v1/pantry')
    expect(s.items).toHaveLength(1)
    expect(s.reviewedAt).toBe('2026-10-01T10:00:00Z')
    expect(s.loaded).toBe(true)
  })

  it('adds via POST and shows the returned row, merging by id', async () => {
    mockFetch(200, { items: [row()], reviewedAt: null })
    const s = usePantry()
    await s.load()
    const spy = mockFetch(201, row({ quantity: 1000 }))
    await s.add({ name: 'Ryż', quantity: 500, unit: 'g', expiresOn: '', staple: false })
    expect(spy.mock.calls[0][0]).toBe('/api/v1/pantry')
    expect(spy.mock.calls[0][1]!.method).toBe('POST')
    expect(JSON.parse(spy.mock.calls[0][1]!.body as string).name).toBe('Ryż')
    expect(s.items).toHaveLength(1)
    expect(s.items[0].quantity).toBe(1000)
  })

  it('sends null quantity for staples and empty input', async () => {
    const s = usePantry()
    const spy = mockFetch(201, row({ quantity: null, staple: true }))
    await s.add({ productId: 'p', quantity: '' as unknown as number, unit: 'g', expiresOn: '', staple: true })
    expect(JSON.parse(spy.mock.calls[0][1]!.body as string).quantity).toBeNull()
  })

  it('updates with PUT and replaces the row', async () => {
    mockFetch(200, { items: [row()], reviewedAt: null })
    const s = usePantry()
    await s.load()
    const spy = mockFetch(200, row({ quantity: 550 }))
    await s.update('a', { quantity: 550, unit: 'g', expiresOn: '', staple: false })
    expect(spy.mock.calls[0][0]).toBe('/api/v1/pantry/a')
    expect(spy.mock.calls[0][1]!.method).toBe('PUT')
    expect(JSON.parse(spy.mock.calls[0][1]!.body as string)).toEqual({ quantity: 550, unit: 'g', expiresOn: '', staple: false })
    expect(s.items[0].quantity).toBe(550)
  })

  it('removes with DELETE', async () => {
    mockFetch(200, { items: [row()], reviewedAt: null })
    const s = usePantry()
    await s.load()
    const spy = mockFetch(204, null)
    await s.remove('a')
    expect(spy.mock.calls[0][0]).toBe('/api/v1/pantry/a')
    expect(spy.mock.calls[0][1]!.method).toBe('DELETE')
    expect(s.items).toEqual([])
  })

  it('marks reviewed', async () => {
    const spy = mockFetch(200, { reviewedAt: '2026-11-08T10:00:00Z' })
    const s = usePantry()
    await s.markReviewed()
    expect(spy.mock.calls[0][0]).toBe('/api/v1/pantry/review')
    expect(spy.mock.calls[0][1]!.method).toBe('POST')
    expect(s.reviewedAt).toBe('2026-11-08T10:00:00Z')
  })
})
