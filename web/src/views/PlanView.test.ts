import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createRouter, createMemoryHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import PlanView from './PlanView.vue'
import { buttonByText, clickable, fakeApi, flush, mountWith, openDialog, setViewportWidth, type Routes } from '../test/mount'
import { errorText, t } from '../messages'
import { dayLine } from '../nutrition'
import { formatDay, SLOTS } from '../weeks'
import { useSession } from '../stores/session'
import type { DaySummary, PlanEntry } from '../api/types'

const api = vi.hoisted(() => ({ current: (async () => ({})) as (m: string, p: string, b?: unknown) => Promise<unknown> }))
vi.mock('../api/client', () => ({
  api: (m: string, p: string, b?: unknown) => api.current(m, p, b),
  ApiError: class extends Error {
    constructor(public status: number, public code: string, message: string, public retryAfter?: number) {
      super(message)
    }
  },
}))

const today = '2026-11-11' // a Wednesday; the week starts on 2026-11-09
const entries: PlanEntry[] = [
  { id: 'p1', date: '2026-11-09', slot: 'dinner', recipeId: 'r1', freeText: '', servings: 4, cookUserId: 'u1', done: false, kind: 'meal' },
  { id: 'p2', date: '2026-11-11', slot: 'dinner', recipeId: 'r2', freeText: '', servings: 2, cookUserId: null, done: false, kind: 'prep', prepDishId: 'd1' },
  { id: 'p3', date: '2026-11-12', slot: 'dinner', recipeId: null, freeText: 'Pizza', servings: 2, cookUserId: null, done: false, kind: 'out' },
]

const macros = (kcal: number, protein: number) => ({ kcal, protein, carbs: 0, fat: 0 })
function day(date: string, kcal: number, protein: number, missingMeals: number): DaySummary {
  return {
    date,
    total: macros(kcal, protein),
    goal: macros(2200, 100),
    remaining: macros(2200 - kcal, 100 - protein),
    meals: [],
    log: [],
    missingMeals,
    missingLog: 0,
    incomplete: { kcal: false, protein: false, carbs: false, fat: false },
    approximate: false,
  }
}

function routes(over: Routes = {}): Routes {
  return {
    'GET /recipes': () => [{ id: 'r1', title: 'Gulasz', tags: [] }, { id: 'r2', title: 'Zupa ogórkowa', tags: [] }],
    'GET /household/members': () => [{ id: 'u1', displayName: 'Marc' }],
    'GET /plan/2026-11-09': () => ({ week: '2026-11-09', entries }),
    'GET /plan/2026-11-16': () => ({ week: '2026-11-16', entries: [] }),
    'GET /plan/2026-11-02': () => ({ week: '2026-11-02', entries: [] }),
    'POST /plan/2026-11-16/copy': () => ({}),
    'GET /me/nutrition': () => ({ enabled: false }),
    'GET /prep/sessions': () => ({ sessions: [] }),
    'GET /freezer': () => ({ items: [] }),
    'GET /household/settings': () => ({ portionsPerMeal: 2 }),
    ...over,
  }
}

async function show(r: Routes) {
  const f = fakeApi(r)
  api.current = f.fn
  const pinia = createPinia()
  setActivePinia(pinia)
  useSession().$patch({ me: { user: { id: 'u1', login: 'marc', displayName: 'Marc' }, household: { id: 'h1', name: 'Dom' }, role: 'admin' } as never, loaded: true })
  const router = createRouter({ history: createMemoryHistory(), routes: ['setup', 'login', 'invite', 'home', 'plan', 'prep', 'recipes', 'recipe-import', 'recipe-new', 'recipe', 'recipe-edit', 'shopping', 'pantry', 'account', 'products', 'members'].map((name) => ({ name, path: `/${name}/:id?`, component: { template: '<div/>' } })) })
  await router.push('/plan')
  const w = mountWith(PlanView, { global: { plugins: [router] } }, pinia)
  await settle()
  return { w, calls: f.calls }
}

async function settle() {
  for (let i = 0; i < 5; i++) await flush()
}
const h1 = () => document.querySelector('h1')?.textContent ?? ''
const click = (sel: string) => (document.querySelector(sel) as HTMLElement).click()
const hasButton = (text: string) => [...document.querySelectorAll('button')].some((b) => b.textContent?.trim() === text)
const bodyText = () => document.body.textContent ?? ''

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(`${today}T09:00:00`))
})
afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
  setViewportWidth(1024)
})

describe.each([['desktop', 1280], ['phone', 390]])('PlanView on %s', (_name, width) => {
  beforeEach(() => setViewportWidth(width))

  it('switches weeks and returns to this week', async () => {
    const { calls } = await show(routes())
    expect(h1()).toContain(formatDay('2026-11-09'))
    expect(h1()).toContain(formatDay('2026-11-15'))
    expect(hasButton(t.thisWeek)).toBe(false)

    click(`button[title="${t.nextWeek}"]`)
    await settle()
    expect(calls.some((c) => c.method === 'GET' && c.path === '/plan/2026-11-16')).toBe(true)
    expect(h1()).toContain(formatDay('2026-11-16'))
    expect(hasButton(t.thisWeek)).toBe(true)

    buttonByText(t.thisWeek).click()
    await settle()
    expect(h1()).toContain(formatDay('2026-11-09'))
    expect(hasButton(t.thisWeek)).toBe(false)

    click(`button[title="${t.prevWeek}"]`)
    await settle()
    expect(calls.some((c) => c.method === 'GET' && c.path === '/plan/2026-11-02')).toBe(true)
  })

  it('copies the previous week into an empty week', async () => {
    const { calls } = await show(routes())
    click(`button[title="${t.nextWeek}"]`)
    await settle()
    buttonByText(t.copyPrevWeek).click()
    await settle()
    const post = calls.find((c) => c.method === 'POST' && c.path === '/plan/2026-11-16/copy')
    expect(post?.body).toEqual({ fromWeek: '2026-11-09' })
    expect(bodyText()).toContain(t.weekCopied)
  })

  it('blocks copying until the week loads and retries after an error', async () => {
    const err = new Error('boom')
    let n = 0
    await show(routes({
      'GET /plan/2026-11-09': () => {
        if (n++ === 0) throw err
        return { week: '2026-11-09', entries }
      },
    }))
    expect(bodyText()).toContain(errorText(err))
    expect(buttonByText(t.copyPrevWeek).disabled).toBe(true)

    buttonByText(t.retry).click()
    await settle()
    expect(buttonByText(t.copyPrevWeek).disabled).toBe(false)
    expect(bodyText()).not.toContain(errorText(err))
    expect(bodyText()).toContain('Gulasz')
  })

  it('marks today', async () => {
    await show(routes())
    const tags = [...document.querySelectorAll('.today-tag')]
    expect(tags).toHaveLength(1)
    expect(tags[0].textContent).toBe(t.today)
    expect(tags[0].parentElement?.textContent).toContain(formatDay(today))

    click(`button[title="${t.nextWeek}"]`)
    await settle()
    expect(document.querySelector('.today-tag')).toBeNull()
  })

  it('adds a meal with a screen reader description', async () => {
    await show(routes())
    if (width > 600) {
      const dates = ['2026-11-09', '2026-11-10', '2026-11-11', '2026-11-12', '2026-11-13', '2026-11-14', '2026-11-15']
      for (const d of dates) {
        for (const s of SLOTS) {
          expect(document.querySelector(`[aria-label="${t.addEntryTitle}: ${formatDay(d)}, ${s.title}"]`)).not.toBeNull()
        }
      }
      expect(document.querySelectorAll(`[aria-label^="${t.addEntryTitle}: "]`)).toHaveLength(28)
      click(`[aria-label="${t.addEntryTitle}: ${formatDay('2026-11-12')}, ${t.slotNames.breakfast}"]`)
      await settle()
      expect(openDialog()?.textContent).toContain(t.addEntryTitle)
      expect(openDialog()?.textContent).toContain(t.slotNames.breakfast)
    } else {
      const adds = [...document.querySelectorAll<HTMLElement>('button')].filter((b) => (b.getAttribute('aria-label') ?? b.textContent?.trim() ?? '').startsWith(t.addEntryTitle))
      expect(adds).toHaveLength(7)
      adds[3].click()
      await settle()
      expect(openDialog()?.textContent).toContain(t.addEntryTitle)
      expect(openDialog()?.textContent).toContain(formatDay('2026-11-12'))
      expect(openDialog()?.textContent).toContain(t.slotNames.dinner)
    }
  })

  it('opens an entry for editing on click', async () => {
    await show(routes())
    clickable('Gulasz').click()
    await settle()
    expect(openDialog()?.textContent).toContain(t.editEntryTitle)
  })

  it('shows the cook and the entry note', async () => {
    await show(routes())
    expect(bodyText()).toContain('Marc')
    expect(bodyText()).toContain(t.chipPrep)
    expect(bodyText()).toContain(t.chipOut)
    expect(bodyText()).toContain('Pizza')
  })

  it("shows the day's calories with a warning", async () => {
    const day0911 = day('2026-11-09', 1840, 92, 1)
    const day1011 = day('2026-11-10', 0, 0, 0)
    await show(routes({
      'GET /me/nutrition': () => ({ enabled: true, goal: macros(2200, 100) }),
      'GET /me/nutrition/days?from=2026-11-09&to=2026-11-15': () => ({ enabled: true, days: [day0911, day1011] }),
    }))
    expect(bodyText()).toContain(dayLine(day0911))
    expect(bodyText()).not.toContain(dayLine(day1011))
    expect(document.querySelector(`[aria-label="${t.mealsWithoutValues(1)}"]`)).not.toBeNull()
  })
})
