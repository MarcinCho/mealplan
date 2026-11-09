import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createRouter, createMemoryHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import TodayView from './TodayView.vue'
import { buttonByText, fakeApi, flush, mountWith, type Routes } from '../test/mount'
import { t } from '../messages'
import { useSession } from '../stores/session'
import type { PlanEntry, PrepSession } from '../api/types'

const api = vi.hoisted(() => ({ current: (async () => ({})) as (m: string, p: string, b?: unknown) => Promise<unknown> }))
vi.mock('../api/client', () => ({ api: (m: string, p: string, b?: unknown) => api.current(m, p, b), ApiError: class extends Error {} }))

const today = '2026-11-11' // a Wednesday; the week starts on 2026-11-09
const entries: PlanEntry[] = [
  { id: 'e1', date: today, slot: 'lunch', recipeId: 'r1', freeText: '', servings: 2, cookUserId: null, done: false, kind: 'meal' },
  { id: 'e2', date: today, slot: 'dinner', recipeId: 'r1', freeText: '', servings: 2, cookUserId: null, done: false, kind: 'prep', prepDishId: 'd1' },
  { id: 'e3', date: today, slot: 'breakfast', recipeId: null, freeText: '', servings: 2, cookUserId: null, done: true, kind: 'out' },
]

function session(date: string, remaining: number): PrepSession {
  return { id: 's1', date, name: 'Gotowanie', week: '2026-11-09', cooked: true, remaining, dishes: [] }
}

function routes(over: Partial<{ sessions: PrepSession[]; reviewedAt: string | null; items: number }> = {}): Routes {
  return {
    'GET /recipes': () => [{ id: 'r1', title: 'Gulasz', tags: [] }],
    'GET /household/members': () => [{ id: 'u1', displayName: 'Marc' }],
    'GET /plan/2026-11-09': () => ({ week: '2026-11-09', entries }),
    'GET /me/nutrition': () => ({ enabled: false, goal: null }),
    'GET /pantry': () => ({ items: Array.from({ length: over.items ?? 0 }, (_, i) => ({ id: `p${i}`, name: 'x', productId: 'x', quantity: 1, unit: 'g', staple: false, expiresOn: null })), reviewedAt: over.reviewedAt ?? null }),
    'GET /prep/sessions': () => ({ sessions: over.sessions ?? [] }),
    'GET /freezer': () => ({ items: [] }),
    'GET /household/settings': () => ({ portionsPerMeal: 2 }),
    'PUT /plan/2026-11-09/entries/e1': (b) => ({ ...entries[0], ...(b as object) }),
  }
}

async function show(r: Routes) {
  const f = fakeApi(r)
  api.current = f.fn
  const pinia = createPinia()
  setActivePinia(pinia)
  useSession().$patch({ me: { user: { id: 'u1', login: 'marc', displayName: 'Marc' }, household: { id: 'h1', name: 'Dom' }, role: 'admin' } as never, loaded: true })
  const router = createRouter({ history: createMemoryHistory(), routes: ['setup','login','invite','home','plan','prep','recipes','recipe-import','recipe-new','recipe-edit','shopping','pantry','account','products','members'].map((name) => ({ name, path: `/${name}/:id?`, component: { template: '<div/>' } })) })
  const w = mountWith(TodayView, { global: { plugins: [router] } }, pinia)
  for (let i = 0; i < 5; i++) await flush()
  return { w, calls: f.calls }
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(`${today}T09:00:00`))
})
afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
})

describe('TodayView', () => {
  it('lists the day with recipe, prep and eaten-out titles and puts the next meal first', async () => {
    const { w } = await show(routes())
    const titles = w.findAll('.meal-title').map((x) => x.text())
    expect(titles).toEqual([t.entryOut, 'Gulasz', 'Gulasz'])
    expect(w.find('.hero-title').text()).toBe('Gulasz')
    expect(w.text()).toContain('Marc')
  })

  it('marks the hero meal done with the whole entry sent back', async () => {
    const { calls } = await show(routes())
    buttonByText(t.markDone).click()
    for (let i = 0; i < 3; i++) await flush()
    const put = calls.find((c) => c.method === 'PUT')
    expect(put?.path).toBe('/plan/2026-11-09/entries/e1')
    expect(put?.body).toMatchObject({ done: true, recipeId: 'r1', servings: 2, kind: 'meal' })
  })

  it('reminds about the pantry review and leftover portions when they are due', async () => {
    const { w } = await show(routes({ items: 2, reviewedAt: '2026-10-01T10:00:00Z', sessions: [session('2026-11-05', 3)] }))
    expect(w.text()).toContain(t.pantryReviewDue)
    expect(w.text()).toContain(t.prepStaleToday)
  })

  it('stays quiet when the pantry is fresh and the session is recent or used up', async () => {
    const { w } = await show(routes({ items: 2, reviewedAt: '2026-11-01T10:00:00Z', sessions: [session('2026-11-09', 3), { ...session('2026-11-01', 0), id: 's2' }] }))
    expect(w.text()).not.toContain(t.pantryReviewDue)
    expect(w.text()).not.toContain(t.prepStaleToday)
  })
})
