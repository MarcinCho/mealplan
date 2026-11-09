import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { isCovered, useShopping } from './shopping'

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

  it('keeps the previous-week undone count from load and generate', async () => {
    const s = useShopping()
    stub(200, { week: '2026-11-02', items: [], previousWeekUndone: 3 })
    await s.load('2026-11-02')
    expect(s.previousWeekUndone).toBe(3)
    stub(200, { week: '2026-11-02', items: [], previousWeekUndone: 0 })
    await s.generate('2026-11-02')
    expect(s.previousWeekUndone).toBe(0)
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

  it('moveToPantry posts the week, reloads the list and returns the count', async () => {
    const s = useShopping()
    stub(200, { week: '2026-11-02', items: [] })
    await s.load('2026-11-02')
    const spy = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ moved: 2 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ week: '2026-11-02', items: [{ ...item('a', 'Mąka', true), moved: true }] }), { status: 200 }))
    vi.stubGlobal('fetch', spy)
    const n = await s.moveToPantry('2026-11-02')
    expect(n).toBe(2)
    expect(spy.mock.calls[0][0]).toBe('/api/v1/shopping/move-to-pantry')
    expect(JSON.parse((spy.mock.calls[0][1] as RequestInit).body as string)).toEqual({ week: '2026-11-02' })
    expect(spy.mock.calls[1][0]).toBe('/api/v1/shopping?week=2026-11-02')
    expect(s.items[0].moved).toBe(true)
  })

  it('moveToPantry does not reload over another week the user switched to', async () => {
    const s = useShopping()
    let finishMove!: (r: Response) => void
    const spy = vi.fn((url: string) => {
      if (url.includes('move-to-pantry')) return new Promise<Response>((r) => { finishMove = r })
      const week = url.split('week=')[1]
      return Promise.resolve(new Response(JSON.stringify({ week, items: [{ ...item('x-' + week, 'Pozycja'), week }] }), { status: 200 }))
    })
    vi.stubGlobal('fetch', spy)
    await s.load('2026-11-02')
    const moving = s.moveToPantry('2026-11-02')
    await s.load('2026-11-09')
    finishMove(new Response(JSON.stringify({ moved: 1 }), { status: 200 }))
    expect(await moving).toBe(1)
    expect(s.week).toBe('2026-11-09')
    expect(s.items.map((i) => i.id)).toEqual(['x-2026-11-09'])
    expect(spy.mock.calls.filter((c) => !String(c[0]).includes('move')).length).toBe(2)
  })

  it('splits covered, open and done rows', async () => {
    const mk = (id: string, o: object) => ({ ...item(id, id), ...o })
    stub(200, { week: '2026-11-02', items: [
      mk('covered', { quantity: 0, haveQuantity: 500 }),
      mk('coveredChecked', { quantity: 0, haveQuantity: 500, checked: true }),
      mk('zeroHave', { quantity: 0, haveQuantity: 0 }),
      mk('manualZero', { quantity: 0, manual: true }),
      mk('partial', { quantity: 300, haveQuantity: 200 }),
      mk('bought', { quantity: 1, checked: true }),
    ] })
    const s = useShopping()
    await s.load('2026-11-02')
    expect(s.covered.map((i) => i.id)).toEqual(['covered', 'coveredChecked'])
    expect(s.open.map((i) => i.id)).toEqual(['zeroHave', 'manualZero', 'partial'])
    expect(s.done.map((i) => i.id)).toEqual(['bought'])
    expect(isCovered(mk('m', { quantity: 0, haveQuantity: null }))).toBe(false)
  })
})
