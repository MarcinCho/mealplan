import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createRouter, createMemoryHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import TodayView from './TodayView.vue'
import { buttonByText, clickable, fakeApi, flush, mountWith, openDialog, setViewportWidth, type Routes } from '../test/mount'
import { t } from '../messages'
import { portions } from '../polish'
import { useSession } from '../stores/session'
import type { DaySummary, PlanEntry, PrepSession, Recipe } from '../api/types'

const api = vi.hoisted(() => ({ current: (async () => ({})) as (m: string, p: string, b?: unknown) => Promise<unknown> }))
vi.mock('../api/client', () => ({ api: (m: string, p: string, b?: unknown) => api.current(m, p, b), ApiError: class extends Error {} }))

const today = '2026-11-11' // a Wednesday; the week starts on 2026-11-09
const entries: PlanEntry[] = [
  { id: 'e1', date: today, slot: 'lunch', recipeId: 'r1', freeText: '', servings: 4, cookUserId: null, done: false, kind: 'meal' },
  { id: 'e2', date: today, slot: 'dinner', recipeId: 'r1', freeText: '', servings: 2, cookUserId: null, done: false, kind: 'prep', prepDishId: 'd1' },
  { id: 'e3', date: today, slot: 'breakfast', recipeId: null, freeText: '', servings: 2, cookUserId: null, done: true, kind: 'out' },
]

function session(date: string, remaining: number): PrepSession {
  return { id: 's1', date, name: 'Gotowanie', week: '2026-11-09', cooked: true, remaining, dishes: [] }
}

const macros = (kcal: number | null) => ({ kcal, protein: 10, carbs: 20, fat: 5 })
function daySummary(date: string, over: Partial<DaySummary> = {}): DaySummary {
  return {
    date,
    total: macros(500),
    goal: macros(2000),
    remaining: macros(1500),
    meals: [],
    log: [],
    missingMeals: 0,
    missingLog: 0,
    incomplete: { kcal: false, protein: false, carbs: false, fat: false },
    approximate: false,
    ...over,
  }
}

type RouteOpts = Partial<{
  sessions: PrepSession[]
  reviewedAt: string | null
  items: number
  entries: PlanEntry[]
  recipes: Pick<Recipe, 'id' | 'title' | 'tags'>[]
  members: { id: string; displayName: string }[]
  nutrition: { enabled: boolean; days: DaySummary[] }
  failRecipesOnce: boolean
}>

function routes(over: RouteOpts = {}): Routes {
  const list = over.entries ?? entries
  let failed = false
  return {
    'GET /recipes': () => {
      if (over.failRecipesOnce && !failed) {
        failed = true
        throw new Error('boom')
      }
      return over.recipes ?? [{ id: 'r1', title: 'Gulasz', tags: [] }]
    },
    'GET /household/members': () => over.members ?? [{ id: 'u1', displayName: 'Marc' }],
    'GET /plan/2026-11-09': () => ({ week: '2026-11-09', entries: list }),
    'GET /me/nutrition': () => ({ enabled: over.nutrition?.enabled ?? false, goal: null }),
    'GET /me/nutrition/days?from=2026-11-09&to=2026-11-15': () => ({ days: over.nutrition?.days ?? [] }),
    'GET /pantry': () => ({ items: Array.from({ length: over.items ?? 0 }, (_, i) => ({ id: `p${i}`, name: 'x', productId: 'x', quantity: 1, unit: 'g', staple: false, expiresOn: null })), reviewedAt: over.reviewedAt ?? null }),
    'GET /prep/sessions': () => ({ sessions: over.sessions ?? [] }),
    'GET /freezer': () => ({ items: [] }),
    'GET /household/settings': () => ({ portionsPerMeal: 2 }),
    'PUT /plan/2026-11-09/entries/e1': (b) => ({ ...list[0], ...(b as object) }),
    'DELETE /me/food-log/l1': () => ({}),
  }
}

async function show(r: Routes) {
  const f = fakeApi(r)
  api.current = f.fn
  const pinia = createPinia()
  setActivePinia(pinia)
  useSession().$patch({ me: { user: { id: 'u1', login: 'marc', displayName: 'Marc' }, household: { id: 'h1', name: 'Dom' }, role: 'admin' } as never, loaded: true })
  const router = createRouter({ history: createMemoryHistory(), routes: ['setup','login','invite','home','plan','prep','recipes','recipe-import','recipe-new','recipe','recipe-edit','shopping','pantry','account','products','members'].map((name) => ({ name, path: `/${name}/:id?`, component: { template: '<div/>' } })) })
  await router.push('/home')
  const w = mountWith(TodayView, { global: { plugins: [router] } }, pinia)
  for (let i = 0; i < 5; i++) await flush()
  return { w, calls: f.calls }
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(`${today}T09:00:00`))
})
afterEach(() => {
  setViewportWidth(1024)
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
    expect(put?.body).toMatchObject({ done: true, recipeId: 'r1', servings: 4, kind: 'meal' })
  })

  it('opens the recipe reading view at the planned servings', async () => {
    await show(routes())
    const link = [...document.querySelectorAll('a')].find((a) => a.textContent?.trim() === t.openRecipe)
    expect(link?.getAttribute('href')).toContain('/recipe/r1')
    expect(link?.getAttribute('href')).toContain('servings=4')
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

  it('offers a retry when loading fails', async () => {
    const { w } = await show(routes({ failRecipesOnce: true }))
    expect(w.text()).toContain(t.retry)
    clickable(t.retry).click()
    for (let i = 0; i < 5; i++) await flush()
    expect(w.find('.hero-title').text()).toBe('Gulasz')
  })

  it('shows a progress bar while loading', async () => {
    const { w } = await show({ ...routes(), 'GET /recipes': () => new Promise(() => {}) })
    expect(w.find('.v-progress-linear').exists()).toBe(true)
  })

  it('shows the finished and the empty hero', async () => {
    const done = await show(routes({ entries: entries.map((e) => ({ ...e, done: true })) }))
    expect(done.w.text()).toContain(t.allCooked)
    done.w.unmount()
    const empty = await show(routes({ entries: [] }))
    expect(empty.w.text()).toContain(t.nothingPlanned)
    expect(empty.w.text()).toContain(t.nothingPlannedHint)
  })

  it('switches the day with the strip', async () => {
    const { w } = await show(routes())
    const days = w.findAll('button[aria-pressed]')
    expect(days).toHaveLength(7)
    expect(days[2].attributes('aria-pressed')).toBe('true')
    expect(days[2].find('i:not(.off)').exists()).toBe(true)
    expect(days[3].find('i.off').exists()).toBe(true)
    await days[3].trigger('click')
    expect(days[3].attributes('aria-pressed')).toBe('true')
    expect(w.text()).toContain(t.nothingPlanned)
  })

  it('opens a meal for editing', async () => {
    const { w } = await show(routes())
    await w.find('.meal-title').trigger('click')
    await flush()
    expect(openDialog()?.textContent).toContain(t.editEntryTitle)
  })

  it('toggles a meal without opening it', async () => {
    const { calls } = await show(routes())
    document.querySelector<HTMLElement>(`button[aria-label="${t.markDone}: Gulasz"]`)!.click()
    for (let i = 0; i < 3; i++) await flush()
    const put = calls.find((c) => c.method === 'PUT')
    expect(put?.path).toBe('/plan/2026-11-09/entries/e1')
    expect(put?.body).toMatchObject({ done: true })
    expect(document.querySelector('.v-overlay--active')).toBeNull()
  })

  it('opens the meal nutrition from the calorie chip', async () => {
    const meal = { entryId: 'e1', title: 'Gulasz', slot: 'lunch' as const, values: macros(500), sources: { kcal: 'recipe', protein: 'recipe', carbs: 'recipe', fat: 'recipe' } as const, approximate: false, missing: false }
    await show(routes({ nutrition: { enabled: true, days: [daySummary(today, { meals: [meal] })] } }))
    document.querySelector<HTMLElement>(`[title="${t.mealNutritionTitle}"]`)!.click()
    await flush()
    const dialog = openDialog()
    expect(dialog).not.toBeNull()
    expect(dialog?.textContent).not.toContain(t.editEntryTitle)
  })

  it('shows who cooks', async () => {
    const { w } = await show(routes({ entries: entries.map((e) => (e.id === 'e1' ? { ...e, cookUserId: 'u1' } : e)) }))
    const badge = w.find('[title="Marc"]')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toContain('M')
  })

  it('skips an unknown cook', async () => {
    const { w } = await show(routes({ entries: entries.map((e) => (e.id === 'e1' ? { ...e, cookUserId: 'gone' } : e)) }))
    expect(w.text()).not.toContain('undefined')
    expect(w.findAll('[title="Marc"]')).toHaveLength(0)
  })

  it('adds a meal in a free slot', async () => {
    await show(routes({ entries: entries.filter((e) => e.slot !== 'dinner') }))
    clickable(`${t.addMeal}: ${t.slotNames.dinner.toLowerCase()}`).click()
    await flush()
    expect(openDialog()?.textContent).toContain(t.addEntryTitle)
  })

  it('logs food in the day summary', async () => {
    const log = [{ id: 'l1', productId: 'p1', name: 'Jabłko', quantity: 100, unit: 'g' as const, values: macros(52), missing: false }]
    const { w, calls } = await show(routes({ nutrition: { enabled: true, days: [daySummary(today, { log })] } }))
    expect(w.text()).toContain(t.daySummary)
    buttonByText(t.logAdd).click()
    await flush()
    expect(openDialog()).not.toBeNull()
    document.body.querySelectorAll('.v-overlay--active').forEach((o) => o.remove())
    document.querySelector<HTMLElement>(`button[title="${t.logRemove}"]`)!.click()
    for (let i = 0; i < 3; i++) await flush()
    expect(calls.some((c) => c.method === 'DELETE' && c.path === '/me/food-log/l1')).toBe(true)
  })

  it('shows the empty state without recipes', async () => {
    const { w } = await show(routes({ recipes: [], entries: [] }))
    expect(w.text()).toContain(t.noRecipes)
    expect([...document.querySelectorAll('a')].some((a) => a.getAttribute('href')?.includes('/recipe-new'))).toBe(true)
  })

  it('offers quick add', async () => {
    await show(routes())
    expect(document.querySelector(`input[placeholder="${t.quickPlaceholder}"]`)).not.toBeNull()
  })

  it('shows the side panel only on desktop', async () => {
    setViewportWidth(1280)
    const wide = await show(routes())
    expect(wide.w.text()).toContain(t.cooksThisWeek)
    expect([...document.querySelectorAll('a')].some((a) => a.textContent?.trim() === t.openShopping)).toBe(true)
    wide.w.unmount()
    setViewportWidth(390)
    const narrow = await show(routes())
    expect(narrow.w.text()).not.toContain(t.cooksThisWeek)
    expect([...document.querySelectorAll('a')].some((a) => a.textContent?.trim() === t.openShopping)).toBe(false)
  })

  it('shows the next meal in a note card', async () => {
    const { w } = await show(routes())
    const card = w.find('.note-card')
    expect(card.find('.note-card__label').text()).toBe(`${t.todayOn} ${t.slotOn.lunch}`)
    expect(card.find('.hero-title').text()).toBe('Gulasz')
    expect(card.text()).toContain(portions(4))
    expect(card.findAll('button').some((b) => b.text() === t.markDone)).toBe(true)
    expect(card.findAll('a').some((a) => a.text() === t.openRecipe)).toBe(true)
  })

  it('lists the day as ruled rows with the meal time on the left', async () => {
    // freeSlots offers the main meals only (snacks stay optional), so the free row here is the dinner.
    const { w } = await show(routes({ entries: entries.filter((e) => e.slot !== 'dinner') }))
    const rows = w.findAll('.note-row').filter((r) => r.find('.meal-title').exists())
    expect(rows.map((r) => r.find('.note-row__label').text())).toEqual([t.slotNames.breakfast, t.slotNames.lunch])
    const free = w.findAll('.note-row').find((r) => r.find('.note-row__label').text() === t.slotNames.dinner)
    expect(free?.findAll('button').some((b) => b.text().includes(t.addMeal))).toBe(true)
    expect(w.find('.meal-title').element.tagName).toBe('BUTTON')
  })

  it('shows this week as rows with today highlighted', async () => {
    setViewportWidth(1280)
    const { w } = await show(routes())
    expect(w.findAll('aside .note-row')).toHaveLength(7)
    const hot = w.findAll('aside .note-row--highlighted')
    expect(hot).toHaveLength(1)
    expect(hot[0].attributes('aria-current')).toBe('date')
    expect(hot[0].text()).toContain(t.dayNames[2])
    expect(hot[0].text()).toContain('11')
    expect(hot[0].text()).toContain('Gulasz')
  })

  it('has no side panel on a phone', async () => {
    setViewportWidth(390)
    const { w } = await show(routes())
    expect(w.find('aside').exists()).toBe(false)
    expect(w.findAll('button[aria-pressed]')).toHaveLength(7)
  })
})
