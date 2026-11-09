import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { defineComponent } from 'vue'
import RecipeEditView from './RecipeEditView.vue'
import { buttonByText, fakeApi, flush, mountWith, type Routes } from '../test/mount'
import { t } from '../messages'

const api = vi.hoisted(() => ({ current: (async () => ({})) as (m: string, p: string, b?: unknown) => Promise<unknown> }))
vi.mock('../api/client', () => ({ api: (m: string, p: string, b?: unknown) => api.current(m, p, b), ApiError: class extends Error {} }))

const Stub = defineComponent({ render: () => null })

async function show(path: string, r: Routes) {
  const f = fakeApi(r)
  api.current = f.fn
  const pinia = createPinia()
  setActivePinia(pinia)
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
    expect(router.currentRoute.value.name).toBe('recipe-edit')
  })
})
