import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import EntryDialog from './EntryDialog.vue'
import { buttonByText, flush, mountWith } from '../test/mount'
import { t } from '../messages'
import { useMealPrepStore } from '../stores/mealprep'
import type { PlanEntry, PrepSession, Recipe } from '../api/types'

vi.mock('../api/client', () => ({ api: vi.fn(async () => ({ sessions: [], items: [], portionsPerMeal: 2 })), ApiError: class extends Error {} }))

const dates = ['2026-11-09', '2026-11-10']
const recipes = [{ id: 'r1', title: 'Gulasz' } as Recipe]

function entry(over: Partial<PlanEntry>): PlanEntry {
  return { id: 'e1', date: '2026-11-10', slot: 'dinner', recipeId: null, freeText: '', servings: 2, cookUserId: null, done: false, ...over }
}

const session: PrepSession = {
  id: 's1', date: '2026-11-08', name: 'Niedziela', week: '2026-11-09', cooked: true, remaining: 3,
  dishes: [{ id: 'd1', recipeId: 'r1', recipeTitle: 'Gulasz', portions: 8, writtenOff: 0, allocated: 5, frozen: 0, remaining: 3, allocations: [] }],
}

function open(e: PlanEntry | null) {
  const pinia = createPinia()
  setActivePinia(pinia)
  useMealPrepStore().$patch({ sessions: [session], freezer: [], loaded: true, portionsPerMeal: 2 })
  return mountWith(EntryDialog, { props: { modelValue: true, dates, recipes, members: [], entry: e, initial: { date: dates[0], slot: 'lunch' } } }, pinia)
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('EntryDialog', () => {
  it('saves an edited recipe meal with its fields', async () => {
    const w = open(entry({ recipeId: 'r1', servings: 4, done: true }))
    await flush()
    buttonByText(t.save).click()
    expect(w.emitted('save')?.[0][0]).toEqual({ date: '2026-11-10', slot: 'dinner', done: true, kind: 'meal', recipeId: 'r1', freeText: '', servings: 4, cookUserId: null })
  })

  it('keeps a prep entry as prep and sends its dish', async () => {
    const w = open(entry({ kind: 'prep', recipeId: 'r1', prepDishId: 'd1', servings: 2 }))
    await flush()
    buttonByText(t.save).click()
    expect(w.emitted('save')?.[0][0]).toMatchObject({ kind: 'prep', prepDishId: 'd1', freezerItemId: null, servings: 2 })
  })

  it('refuses more prep portions than are left', async () => {
    // own 2 plus 3 remaining: 5 is the most this entry may take
    const w = open(entry({ kind: 'prep', recipeId: 'r1', prepDishId: 'd1', servings: 2 }))
    await flush()
    const servings = [...document.querySelectorAll('input[type="number"]')][0] as HTMLInputElement
    servings.value = '6'
    servings.dispatchEvent(new Event('input'))
    await flush()
    expect(buttonByText(t.save).disabled).toBe(true)
    servings.value = '5'
    servings.dispatchEvent(new Event('input'))
    await flush()
    expect(buttonByText(t.save).disabled).toBe(false)
    w.unmount()
  })

  it('sends eaten-out values as numbers and blanks as null', async () => {
    const w = open(entry({ kind: 'out', freeText: 'Pizza', outValues: { kcal: 900, protein: null, carbs: 100, fat: null } }))
    await flush()
    buttonByText(t.save).click()
    expect(w.emitted('save')?.[0][0]).toMatchObject({ kind: 'out', freeText: 'Pizza', outValues: { kcal: 900, protein: null, carbs: 100, fat: null } })
  })

  it('cannot save a new recipe meal without a recipe', async () => {
    open(null)
    await flush()
    expect(buttonByText(t.save).disabled).toBe(true)
  })
})
