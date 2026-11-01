import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useShopping } from './shopping'

function stub(status: number, body: unknown) {
  const spy = vi.fn(async (_url: string, _init?: RequestInit) => new Response(status === 204 ? null : JSON.stringify(body), { status }))
  vi.stubGlobal('fetch', spy)
  return spy
}

const item = (id: string, name: string, checked = false) => ({
  id, week: '2026-11-02', name, quantity: null, unit: '', checked, manual: false,
})

describe('useShopping', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('loads a week and splits open and done', async () => {
    const spy = stub(200, { week: '2026-11-02', items: [item('a', 'Mąka'), item('b', 'Sól', true)] })
    const s = useShopping()
    await s.load('2026-11-02')
    expect(spy.mock.calls[0][0]).toBe('/api/v1/shopping?week=2026-11-02')
    expect(s.open.map((i) => i.id)).toEqual(['a'])
    expect(s.done.map((i) => i.id)).toEqual(['b'])
  })

  it('generate replaces the list with the server result', async () => {
    const s = useShopping()
    const spy = stub(200, { week: '2026-11-02', items: [item('a', 'Mąka')] })
    await s.generate('2026-11-02')
    expect(spy.mock.calls[0][0]).toBe('/api/v1/shopping/generate')
    expect(JSON.parse((spy.mock.calls[0][1] as RequestInit).body as string)).toEqual({ week: '2026-11-02' })
    expect(s.items).toHaveLength(1)
  })

  it('checks optimistically and rolls back when the request fails', async () => {
    stub(200, { week: '2026-11-02', items: [item('a', 'Mąka')] })
    const s = useShopping()
    await s.load('2026-11-02')
    stub(500, { code: 'internal', message: 'x' })
    await expect(s.setChecked('a', true)).rejects.toThrow()
    expect(s.items[0].checked).toBe(false)
  })

  it('keeps the check when the request succeeds', async () => {
    stub(200, { week: '2026-11-02', items: [item('a', 'Mąka')] })
    const s = useShopping()
    await s.load('2026-11-02')
    stub(204, null)
    await s.setChecked('a', true)
    expect(s.done.map((i) => i.id)).toEqual(['a'])
  })

  it('removes an item after DELETE', async () => {
    stub(200, { week: '2026-11-02', items: [item('a', 'Mąka')] })
    const s = useShopping()
    await s.load('2026-11-02')
    stub(204, null)
    await s.remove('a')
    expect(s.items).toEqual([])
  })

  it('adds a manual item with the current week', async () => {
    stub(200, { week: '2026-11-02', items: [] })
    const s = useShopping()
    await s.load('2026-11-02')
    const spy = stub(201, { ...item('m', 'Papier'), manual: true })
    await s.addManual('Papier', null, '')
    expect(JSON.parse((spy.mock.calls[0][1] as RequestInit).body as string)).toMatchObject({ week: '2026-11-02', name: 'Papier' })
    expect(s.items[0].manual).toBe(true)
  })

  it('sends an empty quantity as null', async () => {
    stub(200, { week: '2026-11-02', items: [] })
    const s = useShopping()
    await s.load('2026-11-02')
    const spy = stub(201, { ...item('m', 'Papier'), manual: true })
    await s.addManual('Papier', '' as unknown as number, '')
    expect(JSON.parse((spy.mock.calls[0][1] as RequestInit).body as string).quantity).toBeNull()
  })

  it('ignores a stale load result when another week was requested since', async () => {
    const s = useShopping()
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    vi.stubGlobal('fetch', vi.fn(async () => {
      await gate
      return new Response(JSON.stringify({ week: '2026-11-02', items: [item('a', 'Mąka')] }), { status: 200 })
    }))
    const first = s.load('2026-11-02')
    stub(200, { week: '2026-11-09', items: [] })
    await s.load('2026-11-09')
    release()
    await first
    expect(s.week).toBe('2026-11-09')
    expect(s.items).toEqual([])
  })
})
