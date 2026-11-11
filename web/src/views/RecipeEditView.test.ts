import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { defineComponent } from 'vue'
import RecipeEditView from './RecipeEditView.vue'
import ProductPicker from '../components/ProductPicker.vue'
import ProductDialog from '../components/ProductDialog.vue'
import { buttonByText, fakeApi, flush, mountWith, type Routes } from '../test/mount'
import { t } from '../messages'
import { useNotice } from '../stores/notice'
import { useImport } from '../stores/import'
import { ApiError } from '../api/client'
import type { RecipeDraft } from '../api/types'

const api = vi.hoisted(() => ({ current: (async () => ({})) as (m: string, p: string, b?: unknown) => Promise<unknown> }))
vi.mock('../api/client', () => ({ api: (m: string, p: string, b?: unknown) => api.current(m, p, b), ApiError: class extends Error {} }))

const Stub = defineComponent({ render: () => null })

function draftWith(name: string): RecipeDraft {
  return {
    title: 'Zupa',
    servings: 4,
    sourceUrl: '',
    tags: [],
    ingredients: [{ name, quantity: 1, unit: 'szt', raw: `1 szt ${name}`, note: '', confidence: 'ok' }],
    steps: [],
    nutrition: { kcal: null, protein: null, carbs: null, fat: null },
    warnings: [],
  }
}

async function show(path: string, r: Routes, draft?: RecipeDraft) {
  const f = fakeApi(r)
  api.current = f.fn
  const pinia = createPinia()
  setActivePinia(pinia)
  if (draft) useImport().setDraft(draft)
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/recipes', name: 'recipes', component: Stub },
      { path: '/recipes/new', name: 'recipe-new', component: RecipeEditView },
      { path: '/recipes/:id', name: 'recipe-edit', component: RecipeEditView },
    ],
  })
  await router.push(path)
  await router.isReady()
  const w = mountWith(RecipeEditView, { global: { plugins: [router] } }, pinia)
  for (let i = 0; i < 4; i++) await flush()
  return { w, calls: f.calls, router }
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('RecipeEditView', () => {
  it('opens an empty form for a new recipe and saves it', async () => {
    const { w, calls, router } = await show('/recipes/new', {
      'GET /products': () => [],
      'POST /recipes': (b) => ({ ...(b as object), id: 'r1' }),
      'GET /recipes/r1': () => ({ id: 'r1', title: 'Zupa', baseServings: 4, sourceUrl: '', tags: [], ingredients: [], steps: [] }),
    })
    expect(w.text()).toContain(t.newRecipe)
    const inputs = document.querySelectorAll('input')
    expect(inputs.length).toBeGreaterThanOrEqual(5) // title, servings, url, tags + 4 nutrition fields
    const title = w.find('input').element as HTMLInputElement
    title.value = 'Zupa'
    title.dispatchEvent(new Event('input'))
    await flush()
    buttonByText(t.save).click()
    for (let i = 0; i < 4; i++) await flush()
    const post = calls.find((c) => c.method === 'POST' && c.path === '/recipes')
    expect(post).toBeTruthy()
    expect((post!.body as { title: string }).title).toBe('Zupa')
    expect(router.currentRoute.value.name).toBe('recipes')
    expect(useNotice().open).toBe(true)
    expect(useNotice().text).toBe('Zapisano przepis „Zupa”')
  })

  it('closes the form after editing and names the saved recipe', async () => {
    const leczo = { id: 'r2', title: 'Leczo', baseServings: 4, sourceUrl: '', tags: [], ingredients: [], steps: [] }
    const { calls, router } = await show('/recipes/r2', {
      'GET /products': () => [],
      'GET /recipes/r2': () => leczo,
      'PUT /recipes/r2': (b) => ({ ...leczo, ...(b as object) }),
    })
    buttonByText(t.save).click()
    for (let i = 0; i < 4; i++) await flush()
    expect(calls.some((c) => c.method === 'PUT' && c.path === '/recipes/r2')).toBe(true)
    expect(router.currentRoute.value.name).toBe('recipes')
    expect(useNotice().text).toBe('Zapisano przepis „Leczo”')
  })

  it('stays on the form and shows the error when saving fails', async () => {
    const { w, router } = await show('/recipes/new', {
      'GET /products': () => [],
      'POST /recipes': () => {
        throw new Error('boom')
      },
    })
    const title = w.find('input').element as HTMLInputElement
    title.value = 'Zupa'
    title.dispatchEvent(new Event('input'))
    await flush()
    buttonByText(t.save).click()
    for (let i = 0; i < 4; i++) await flush()
    expect(router.currentRoute.value.name).toBe('recipe-new')
    expect(useNotice().open).toBe(false)
  })

  it('lets an ingredient add a missing product, prefilled with its name', async () => {
    const leczo = { id: 'r2', title: 'Leczo', baseServings: 4, sourceUrl: '', tags: [], ingredients: [{ name: 'papryka', quantity: 2, unit: '', note: '', productId: null }], steps: [] }
    const { w } = await show('/recipes/r2', { 'GET /products': () => [], 'GET /recipes/r2': () => leczo })
    expect(w.findComponent(ProductPicker).props('allowCreate')).toBe(true)
    expect(w.findComponent(ProductDialog).props('initialName')).toBe('papryka')
  })

  describe('product suggestions', () => {
    const settings = (configured: boolean) => () => ({ configured, keyHint: 'abcd', model: 'm', defaultModel: 'm' })

    it('suggests a product for an imported ingredient and learns the alias on approve', async () => {
      const { w, calls } = await show(
        '/recipes/new',
        {
          'GET /products': () => [{ id: 'p9', name: 'Cebula', per100g: {}, aliases: [] }],
          'GET /household/assistant': settings(true),
          'POST /assistant/ingredients': () => ({
            suggestions: [{ name: 'czerwona cebula', kind: 'existing', productId: 'p9', productName: 'Cebula', reason: 'model' }],
          }),
          'POST /products/p9/assign': () => ({ updated: 0, aliasAdded: true, aliasConflict: '' }),
        },
        draftWith('czerwona cebula'),
      )
      expect(w.text()).toContain('Propozycja: Cebula')
      buttonByText('Zatwierdź').click()
      for (let i = 0; i < 4; i++) await flush()
      const assign = calls.find((c) => c.path === '/products/p9/assign')
      expect(assign?.body).toEqual({ name: 'czerwona cebula', recipeIds: [] })
      expect(w.findComponent(ProductPicker).props('modelValue')).toBe('p9')
      expect(w.text()).not.toContain('Propozycja')
    })

    it('applies an exact match without asking', async () => {
      const { w } = await show(
        '/recipes/new',
        {
          'GET /products': () => [{ id: 'p9', name: 'Cebula', per100g: {}, aliases: [] }],
          'GET /household/assistant': settings(true),
          'POST /assistant/ingredients': () => ({
            suggestions: [{ name: 'cebula', kind: 'existing', productId: 'p9', productName: 'Cebula', reason: 'exact' }],
          }),
        },
        draftWith('cebula'),
      )
      expect(w.findComponent(ProductPicker).props('modelValue')).toBe('p9')
      expect(w.text()).not.toContain('Zatwierdź')
    })

    it('keeps the form usable when suggestions fail', async () => {
      const { w, calls, router } = await show(
        '/recipes/new',
        {
          'GET /products': () => [],
          'GET /household/assistant': settings(true),
          'POST /assistant/ingredients': () => {
            throw Object.assign(new ApiError(502, 'assistant_unavailable', 'x'))
          },
          'POST /recipes': (b) => ({ ...(b as object), id: 'r1' }),
        },
        draftWith('cebula'),
      )
      expect(w.text()).toContain('Podpowiedzi produktów są teraz niedostępne')
      buttonByText(t.save).click()
      for (let i = 0; i < 4; i++) await flush()
      expect(calls.some((c) => c.method === 'POST' && c.path === '/recipes')).toBe(true)
      expect(router.currentRoute.value.name).toBe('recipes')
    })

    it('does not call the assistant when it is not configured', async () => {
      const { calls } = await show(
        '/recipes/new',
        { 'GET /products': () => [], 'GET /household/assistant': settings(false) },
        draftWith('cebula'),
      )
      expect(calls.some((c) => c.path === '/assistant/ingredients')).toBe(false)
    })

    it('opens the product dialog prefilled for a new-product suggestion', async () => {
      const { w } = await show(
        '/recipes/new',
        {
          'GET /products': () => [],
          'GET /household/assistant': settings(true),
          'POST /assistant/ingredients': () => ({
            suggestions: [
              {
                name: 'tofu',
                kind: 'new',
                newProduct: { name: 'Tofu', per100g: { kcal: 120, protein: 12, carbs: 2, fat: 7 }, pieceGrams: null, densityGPerMl: null, aliases: ['tofu naturalne'] },
                reason: 'model',
              },
            ],
          }),
        },
        draftWith('tofu'),
      )
      expect(w.text()).toContain('Propozycja: Tofu (nowy produkt)')
      buttonByText('Zatwierdź').click()
      await flush()
      const dlg = w.findAllComponents(ProductDialog).find((d) => d.props('initial'))
      expect(dlg?.props('modelValue')).toBe(true)
      expect(dlg?.props('initial')?.per100g?.kcal).toBe(120)
      expect(document.body.textContent).toContain(t.suggestionEstimate)
    })
  })
})
