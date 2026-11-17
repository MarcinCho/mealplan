import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { defineComponent } from 'vue'
import RecipeView from './RecipeView.vue'
import { fakeApi, flush, mountWith, setViewportWidth, type Routes } from '../test/mount'
import { errorText, t } from '../messages'
import { ApiError } from '../api/client'

const api = vi.hoisted(() => ({ current: (async () => ({})) as (m: string, p: string, b?: unknown) => Promise<unknown> }))
vi.mock('../api/client', () => ({
  api: (m: string, p: string, b?: unknown) => api.current(m, p, b),
  ApiError: class extends Error {
    constructor(public status: number, public code: string, message: string, public retryAfter?: number) {
      super(message)
    }
  },
}))

const Stub = defineComponent({ render: () => null })

const soup = () => ({
  id: 'r1',
  title: 'Zupa ogórkowa',
  baseServings: 2,
  sourceUrl: 'https://example.com/zupa',
  tags: ['zupa', 'obiad'],
  ingredients: [
    { name: 'ziemniak', quantity: 300, unit: 'g', note: '', productId: null },
    { name: 'ogórki kiszone', quantity: 3, unit: '', note: 'starte', productId: null },
    { name: 'masło', quantity: 1, unit: 'łyżka', note: '', productId: null },
    { name: 'sól i pieprz', quantity: null, unit: '', note: 'do smaku', productId: null },
  ],
  steps: ['Obierz ziemniaki.', 'Gotuj 20 minut.'],
  nutritionPerPortion: {
    values: { kcal: 320, protein: 12, carbs: 30, fat: 15 },
    sources: {},
    approximate: true,
  },
})

async function show(path: string, over: Record<string, unknown> = {}, r: Routes = {}, from?: string) {
  const f = fakeApi({
    'GET /recipes/r1': () => ({ ...soup(), ...over }),
    'GET /me/nutrition': () => ({ enabled: true, goal: { kcal: null, protein: null, carbs: null, fat: null } }),
    ...r,
  })
  api.current = f.fn
  const pinia = createPinia()
  setActivePinia(pinia)
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/recipes', name: 'recipes', component: Stub },
      { path: '/recipes/:id', name: 'recipe', component: RecipeView },
      { path: '/recipes/:id/edit', name: 'recipe-edit', component: Stub },
    ],
  })
  if (from) await router.push(from)
  await router.push(path)
  await router.isReady()
  const w = mountWith(RecipeView, { global: { plugins: [router] } }, pinia)
  for (let i = 0; i < 4; i++) await flush()
  return { w, router, calls: f.calls }
}

const lines = (w: { findAll: (s: string) => { text: () => string }[] }) => w.findAll('li.ingredient').map((l) => l.text())
const btn = (label: string) => document.querySelector(`button[aria-label="${label}"]`) as HTMLButtonElement

afterEach(() => {
  document.body.innerHTML = ''
  setViewportWidth(1024)
})

describe('RecipeView', () => {
  it('shows the header', async () => {
    const { w } = await show('/recipes/r1')
    expect(w.find('h1').text()).toBe('Zupa ogórkowa')
    expect(w.text()).toContain('zupa')
    expect(w.text()).toContain('obiad')
    const back = w.find('a[href="/recipes"]')
    expect(back.exists()).toBe(true)
    expect(back.text()).toContain(t.recipes)
  })

  it('shows ingredients one per line at base servings', async () => {
    const { w } = await show('/recipes/r1')
    const l = lines(w)
    expect(l[0]).toBe('300 g ziemniak')
    expect(l[1]).toContain('3 ogórki kiszone')
    expect(l[1]).toContain('starte')
    expect(l[2]).toBe('1 łyżka masło')
    expect(l[3]).toContain('sól i pieprz')
    expect(l[3]).toContain('do smaku')
    expect(w.text()).not.toContain(t.scaledFrom(2))
  })

  it('scales from the servings query', async () => {
    const { w } = await show('/recipes/r1?servings=4')
    const l = lines(w)
    expect(l[0]).toBe('600 g ziemniak')
    expect(l[1]).toContain('6 ogórki kiszone')
    expect(l[2]).toBe('2 łyżki masło')
    expect(l[3]).toMatch(/^sól i pieprz/)
    expect(w.text()).toContain('Przeliczone z 2 porcji')
    expect(w.text()).toContain(t.restoreServings)
  })

  it('plus and minus change amounts and the url', async () => {
    const { w, router } = await show('/recipes/r1')
    btn(t.moreServings).click()
    await flush()
    expect(router.currentRoute.value.query.servings).toBe('3')
    expect(lines(w)[0]).toBe('450 g ziemniak')
    btn(t.fewerServings).click()
    await flush()
    btn(t.fewerServings).click()
    await flush()
    expect(router.currentRoute.value.query.servings).toBe('1')
    expect(lines(w)[0]).toBe('150 g ziemniak')
    expect(btn(t.fewerServings).disabled).toBe(true)
  })

  it('disables plus at the maximum', async () => {
    await show('/recipes/r1?servings=100')
    expect(btn(t.moreServings).disabled).toBe(true)
  })

  it('restore drops the query', async () => {
    const { w, router } = await show('/recipes/r1?servings=4')
    ;(document.querySelector('.restore') as HTMLButtonElement).click()
    await flush()
    expect('servings' in router.currentRoute.value.query).toBe(false)
    expect(lines(w)[0]).toBe('300 g ziemniak')
    expect(w.text()).not.toContain('Przeliczone')
  })

  it('ignores an invalid servings query', async () => {
    for (const q of ['0', 'abc']) {
      const { w } = await show(`/recipes/r1?servings=${q}`)
      expect(lines(w)[0]).toBe('300 g ziemniak')
      expect(w.text()).not.toContain('Przeliczone')
      document.body.innerHTML = ''
    }
  })

  it('base servings in the query shows no scaling note', async () => {
    const { w } = await show('/recipes/r1?servings=2')
    expect(lines(w)[0]).toBe('300 g ziemniak')
    expect(lines(w)[1]).toContain('3 ogórki kiszone')
    expect(lines(w)[2]).toBe('1 łyżka masło')
    expect(w.text()).not.toContain('Przeliczone')
  })

  it('an array servings query falls back to base servings', async () => {
    const { w } = await show('/recipes/r1?servings=3&servings=5')
    expect(lines(w)[0]).toBe('300 g ziemniak')
    expect(w.text()).not.toContain('Przeliczone')
  })

  it('names what the restore button restores and groups the stepper', async () => {
    const { w } = await show('/recipes/r1?servings=4')
    expect((document.querySelector('.restore') as HTMLButtonElement).getAttribute('aria-label')).toBe('Przywróć 2 porcje')
    const group = w.find('[role="group"]')
    expect(w.find(`#${group.attributes('aria-labelledby')}`).text()).toBe(t.servingsLabel)
    expect(t.restoreServingsLabel(1)).toBe('Przywróć 1 porcję')
    expect(t.restoreServingsLabel(5)).toBe('Przywróć 5 porcji')
  })

  it('uses replace for servings changes', async () => {
    const { router } = await show('/recipes/r1', {}, {}, '/recipes')
    btn(t.moreServings).click()
    await flush()
    router.back()
    await flush()
    await flush()
    expect(router.currentRoute.value.name).toBe('recipes')
  })

  describe('nutrition line', () => {
    it('shows kcal and macros', async () => {
      const { w } = await show('/recipes/r1')
      expect(w.find('.nutrition').text()).toContain('ok. 320 kcal')
      expect(w.find('.nutrition').text()).toContain('na porcję')
      expect(w.text()).toContain('12 g')
      expect(w.text()).toContain('30 g')
      expect(w.text()).toContain('15 g')
    })

    it('qualifies a macros-only line as per portion and approximate', async () => {
      const nutritionPerPortion = { values: { kcal: null, protein: 12, carbs: 30, fat: null }, sources: {}, approximate: true }
      const { w } = await show('/recipes/r1', { nutritionPerPortion })
      expect(w.find('.nutrition').text()).toBe('ok. 12 g białka · 30 g węglowodanów na porcję')
    })

    it('is hidden when tracking is off', async () => {
      const { w } = await show('/recipes/r1', {}, { 'GET /me/nutrition': () => ({ enabled: false }) })
      expect(w.text()).not.toContain('kcal')
    })

    it('is hidden when no value is known', async () => {
      const nutritionPerPortion = { values: { kcal: null, protein: null, carbs: null, fat: null }, sources: {}, approximate: false }
      const { w } = await show('/recipes/r1', { nutritionPerPortion })
      expect(w.text()).not.toContain('kcal')
    })

    it('does not block the view when settings fail to load', async () => {
      const { w } = await show('/recipes/r1', {}, {
        'GET /me/nutrition': () => {
          throw new Error('boom')
        },
      })
      expect(w.find('h1').text()).toBe('Zupa ogórkowa')
      expect(w.text()).not.toContain('kcal')
    })
  })

  it('hides the ingredients section when there are none', async () => {
    const { w } = await show('/recipes/r1', { ingredients: [] })
    expect(w.text()).not.toContain(t.ingredients)
    expect(w.find('li.ingredient').exists()).toBe(false)
  })

  it('hides empty sections', async () => {
    const { w } = await show('/recipes/r1', { steps: [], tags: [], sourceUrl: '' })
    expect(w.text()).not.toContain(t.steps)
    expect(w.find('a[target="_blank"]').exists()).toBe(false)
  })

  it('source link', async () => {
    const { w } = await show('/recipes/r1')
    const a = w.find('a[href="https://example.com/zupa"]')
    expect(a.attributes('target')).toBe('_blank')
    expect(a.attributes('rel')).toBe('noopener noreferrer')
    expect(a.text()).toContain('example.com')
  })

  it('edit button', async () => {
    const { w } = await show('/recipes/r1?servings=4')
    const a = w.find('a[href="/recipes/r1/edit"]')
    expect(a.exists()).toBe(true)
    expect(a.text()).toContain(t.editButton)
  })

  describe('notebook layout', () => {
    it('puts the ingredients and the servings stepper in a note card', async () => {
      const { w } = await show('/recipes/r1')
      expect(w.find('.note-card .note-card__header [role="group"]').exists()).toBe(true)
      expect(w.find('.note-card__title').text()).toBe(t.ingredients)
      expect(w.findAll('.note-card li.ingredient')).toHaveLength(4)
    })

    it('shows the scaling note under the card header', async () => {
      const { w } = await show('/recipes/r1?servings=4')
      const sub = w.find('.note-card__subheader')
      expect(sub.text()).toContain(t.scaledFrom(2))
      expect(sub.find('.restore').exists()).toBe(true)
      const plain = await show('/recipes/r1')
      expect(plain.w.find('.note-card__subheader').exists()).toBe(false)
    })

    it('keeps the stepper when there are no ingredients', async () => {
      const { w } = await show('/recipes/r1', { ingredients: [] })
      expect(w.find('[role="group"]').exists()).toBe(true)
      expect(w.find('.note-card__title').exists()).toBe(false)
      expect(w.text()).not.toContain(t.ingredients)
    })

    it('uses two columns on desktop and one on a phone', async () => {
      setViewportWidth(1280)
      const desktop = await show('/recipes/r1')
      expect(desktop.w.find('.recipe-view--columns').exists()).toBe(true)
      expect(desktop.w.find('.recipe-view__aside .note-card').exists()).toBe(true)
      expect(desktop.w.find('.recipe-view__main ol.steps').exists()).toBe(true)
      document.body.innerHTML = ''
      setViewportWidth(390)
      const phone = await show('/recipes/r1')
      expect(phone.w.find('.recipe-view--columns').exists()).toBe(false)
      expect(phone.w.findAll('ol.steps > li')).toHaveLength(2)
    })
  })

  describe('errors', () => {
    it('not found', async () => {
      const { w } = await show('/recipes/r1', {}, {
        'GET /recipes/r1': () => {
          throw new ApiError(404, 'not_found', '')
        },
      })
      expect(w.text()).toContain(t.recipeNotFound)
      expect(w.find('a[href="/recipes"]').exists()).toBe(true)
    })

    it('other error', async () => {
      const e = new Error('boom')
      const { w } = await show('/recipes/r1', {}, {
        'GET /recipes/r1': () => {
          throw e
        },
      })
      expect(w.text()).toContain(errorText(e))
    })
  })
})
