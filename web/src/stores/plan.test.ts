import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { usePlan } from './plan'

function stub(status: number, body: unknown) {
  const spy = vi.fn(async (..._args: unknown[]) => new Response(status === 204 ? null : JSON.stringify(body), { status }))
  vi.stubGlobal('fetch', spy)
  return spy
}

const e1 = { id: 'e1', date: '2026-11-03', slot: 'dinner', recipeId: null, freeText: 'Pizza', servings: 2, cookUserId: null }

describe('usePlan', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('loads a week', async () => {
    const spy = stub(200, { week: '2026-11-02', entries: [e1] })
    const p = usePlan()
    await p.load('2026-11-02')
    expect(p.week).toBe('2026-11-02')
    expect(p.entries).toHaveLength(1)
    expect(p.loaded).toBe(true)
    expect(spy.mock.calls[0][0]).toBe('/api/v1/plan/2026-11-02')
  })

  it('selects entries by day and slot', async () => {
    stub(200, { week: '2026-11-02', entries: [e1, { ...e1, id: 'e2', slot: 'lunch' }] })
    const p = usePlan()
    await p.load('2026-11-02')
    expect(p.entriesFor('2026-11-03', 'dinner').map((e) => e.id)).toEqual(['e1'])
    expect(p.entriesFor('2026-11-04', 'dinner')).toEqual([])
  })

  it('adds, edits and removes through the API and updates the grid', async () => {
    stub(200, { week: '2026-11-02', entries: [] })
    const p = usePlan()
    await p.load('2026-11-02')
    stub(201, e1)
    await p.add({ date: '2026-11-03', slot: 'dinner', freeText: 'Pizza' })
    expect(p.entries).toHaveLength(1)
    stub(200, { ...e1, freeText: 'Makaron' })
    await p.edit('e1', { date: '2026-11-03', slot: 'dinner', freeText: 'Makaron' })
    expect(p.entries[0].freeText).toBe('Makaron')
    stub(204, null)
    await p.remove('e1')
    expect(p.entries).toHaveLength(0)
  })

  it('reloads the target week after copying', async () => {
    stub(200, { week: '2026-11-09', entries: [e1] })
    const p = usePlan()
    p.week = '2026-11-09'
    const spy = stub(200, { copied: 1 })
    await p.copyFrom('2026-11-02')
    const first = spy.mock.calls[0]
    expect(first[0]).toBe('/api/v1/plan/2026-11-09/copy')
    expect(JSON.parse((first[1] as RequestInit).body as string)).toEqual({ fromWeek: '2026-11-02' })
  })

  it('does not reload the old week when the user moved on during a copy', async () => {
    stub(200, { week: '2026-11-09', entries: [] })
    const p = usePlan()
    await p.load('2026-11-09')
    let resolveCopy: (r: Response) => void = () => {}
    const pending = new Promise<Response>((r) => (resolveCopy = r))
    const spy = vi.fn().mockReturnValueOnce(pending)
    vi.stubGlobal('fetch', spy)
    const copying = p.copyFrom('2026-11-02')
    stub(200, { week: '2026-11-16', entries: [] })
    await p.load('2026-11-16')
    const after = vi.fn(async () => new Response(JSON.stringify({ week: '2026-11-09', entries: [e1] }), { status: 200 }))
    vi.stubGlobal('fetch', after)
    resolveCopy(new Response(JSON.stringify({ copied: 1 }), { status: 200 }))
    await copying
    expect(after).not.toHaveBeenCalled()
    expect(p.week).toBe('2026-11-16')
    expect(p.loading).toBe(false)
  })

  it('marks the week not loaded and clears entries when a load fails', async () => {
    stub(200, { week: '2026-11-02', entries: [e1] })
    const p = usePlan()
    await p.load('2026-11-02')
    stub(500, { error: 'boom' })
    await expect(p.load('2026-11-09')).rejects.toBeTruthy()
    expect(p.loaded).toBe(false)
    expect(p.entries).toEqual([])
    expect(p.week).toBe('2026-11-09')
  })

  it('ignores a stale response when a newer week was requested', async () => {
    const p = usePlan()
    let resolveFirst: (r: Response) => void = () => {}
    const first = new Promise<Response>((r) => (resolveFirst = r))
    vi.stubGlobal('fetch', vi.fn().mockReturnValueOnce(first))
    const slow = p.load('2026-11-02')
    stub(200, { week: '2026-11-09', entries: [] })
    await p.load('2026-11-09')
    resolveFirst(new Response(JSON.stringify({ week: '2026-11-02', entries: [e1] }), { status: 200 }))
    await slow
    expect(p.week).toBe('2026-11-09')
    expect(p.entries).toEqual([])
    expect(p.loaded).toBe(true)
  })

  it('does not apply a mutation response to a different week', async () => {
    stub(200, { week: '2026-11-02', entries: [] })
    const p = usePlan()
    await p.load('2026-11-02')
    let resolveAdd: (r: Response) => void = () => {}
    const pending = new Promise<Response>((r) => (resolveAdd = r))
    vi.stubGlobal('fetch', vi.fn().mockReturnValueOnce(pending))
    const adding = p.add({ date: '2026-11-03', slot: 'dinner', freeText: 'Pizza' })
    stub(200, { week: '2026-11-09', entries: [] })
    await p.load('2026-11-09')
    resolveAdd(new Response(JSON.stringify(e1), { status: 201 }))
    await adding
    expect(p.entries).toEqual([])
  })
})
