import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { ApiError } from '../api/client'
import { prepErrorText } from '../messages'
import { useMealPrepStore } from './mealprep'

function stubRoutes(extra: Record<string, unknown> = {}) {
  const spy = vi.fn(async (url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`
    const body = key in extra ? extra[key] : key === 'GET /api/v1/prep/sessions' ? { sessions: [{ id: 's1', date: '2026-11-08', name: 'G', week: '2026-11-09', cooked: false, remaining: 3, dishes: [] }] }
      : key === 'GET /api/v1/freezer' ? { items: [{ id: 'f1', title: 'Zupa', portions: 4, allocated: 0, remaining: 4, frozenOn: '2026-11-08', dishId: null, recipeId: null, allocations: [] }] }
      : key === 'GET /api/v1/household/settings' ? { portionsPerMeal: 2 }
      : {}
    return new Response(JSON.stringify(body), { status: 200 })
  })
  vi.stubGlobal('fetch', spy)
  return spy
}
const calls = (spy: ReturnType<typeof stubRoutes>) => spy.mock.calls.map((c) => `${(c[1] as RequestInit | undefined)?.method ?? 'GET'} ${c[0]}`)

describe('useMealPrepStore', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('load fills sessions, freezer and setting', async () => {
    stubRoutes()
    const s = useMealPrepStore()
    await s.load()
    expect(s.sessions).toHaveLength(1)
    expect(s.freezer[0].title).toBe('Zupa')
    expect(s.portionsPerMeal).toBe(2)
  })

  it('freeze posts and reloads sessions and freezer', async () => {
    const spy = stubRoutes()
    const s = useMealPrepStore()
    await s.freeze('d1', 2)
    const c = calls(spy)
    expect(c[0]).toBe('POST /api/v1/prep/dishes/d1/freeze')
    expect(JSON.parse((spy.mock.calls[0][1] as RequestInit).body as string)).toEqual({ portions: 2 })
    expect(c).toContain('GET /api/v1/prep/sessions')
    expect(c).toContain('GET /api/v1/freezer')
    expect(s.sessions).toHaveLength(1)
  })

  it('updateDish and removeFreezer reload', async () => {
    const spy = stubRoutes()
    const s = useMealPrepStore()
    await s.updateDish('d1', 5, 1)
    await s.removeFreezer('f1')
    const c = calls(spy)
    expect(c).toContain('PUT /api/v1/prep/dishes/d1')
    expect(c).toContain('DELETE /api/v1/freezer/f1')
    expect(c.filter((x) => x === 'GET /api/v1/freezer')).toHaveLength(2)
  })

  it('a failed reload after a successful write is recorded, not thrown', async () => {
    const spy = vi.fn(async (url: string, init?: RequestInit) => {
      if ((init?.method ?? 'GET') === 'GET') return new Response(JSON.stringify({ code: 'internal', message: 'boom' }), { status: 500 })
      return new Response('{}', { status: 201 })
    })
    vi.stubGlobal('fetch', spy)
    const s = useMealPrepStore()
    await expect(s.addDish('s1', 'r1', 2)).resolves.toBeUndefined()
    expect(s.reloadError).not.toBe('')
  })

  it('maps in_use deletes to the meal prep message', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ code: 'in_use', message: 'x' }), { status: 409 })))
    const s = useMealPrepStore()
    await expect(s.removeSession('s1')).rejects.toBeInstanceOf(ApiError)
    expect(prepErrorText(await s.removeSession('s1').catch((e) => e))).toMatch(/przydzielone w planie/)
  })
})
