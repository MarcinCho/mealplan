import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { defineComponent } from 'vue'
import RecipesView from './RecipesView.vue'
import { fakeApi, flush, mountWith } from '../test/mount'

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

afterEach(() => {
  document.body.innerHTML = ''
})

describe('RecipesView', () => {
  it('links a recipe row to the reading view', async () => {
    const f = fakeApi({
      'GET /recipes': () => [{ id: 'r1', title: 'Zupa ogórkowa', baseServings: 2, sourceUrl: '', tags: [], ingredients: [], steps: [] }],
      'GET /household/assistant': () => ({ configured: false }),
    })
    api.current = f.fn
    const pinia = createPinia()
    setActivePinia(pinia)
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/recipes', name: 'recipes', component: RecipesView },
        { path: '/recipes/new', name: 'recipe-new', component: Stub },
        { path: '/recipes/import', name: 'recipe-import', component: Stub },
        { path: '/recipes/check', name: 'recipe-check', component: Stub },
        { path: '/recipes/:id', name: 'recipe', component: Stub },
      ],
    })
    await router.push('/recipes')
    await router.isReady()
    const w = mountWith(RecipesView, { global: { plugins: [router] } }, pinia)
    for (let i = 0; i < 4; i++) await flush()
    const row = w.find('a[href="/recipes/r1"]')
    expect(row.exists()).toBe(true)
    expect(row.text()).toContain('Zupa ogórkowa')
  })
})
