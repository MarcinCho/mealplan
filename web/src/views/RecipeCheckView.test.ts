import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import RecipeCheckView from './RecipeCheckView.vue'
import { buttonByText, fakeApi, flush, mountWith, type Routes } from '../test/mount'
import { t } from '../messages'
import { useNotice } from '../stores/notice'

const api = vi.hoisted(() => ({ current: (async () => ({})) as (m: string, p: string, b?: unknown) => Promise<unknown> }))
vi.mock('../api/client', () => ({ api: (m: string, p: string, b?: unknown) => api.current(m, p, b), ApiError: class extends Error {
    constructor(public status: number, public code: string, message: string, public retryAfter?: number) {
      super(message)
    }
  },
}))

const cand = (name: string, over = {}) => ({ name, normalizedName: name, currentProductId: '', recipeIds: ['r1', 'r2'], recipeTitles: ['Zupa', 'Leczo'], ...over })

async function show(r: Routes) {
  const f = fakeApi({
    'GET /household/assistant': () => ({ configured: true, keyHint: 'abcd', model: 'm', defaultModel: 'm' }),
    'GET /products': () => [{ id: 'p0', name: 'Cebula', per100g: {}, aliases: [] }],
    ...r,
  })
  api.current = f.fn
  const pinia = createPinia()
  setActivePinia(pinia)
  const w = mountWith(RecipeCheckView, {}, pinia)
  for (let i = 0; i < 6; i++) await flush()
  return { w, calls: f.calls }
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('RecipeCheckView', () => {
  it('says so when every ingredient has a product', async () => {
    const { w, calls } = await show({ 'GET /assistant/ingredient-candidates': () => ({ candidates: [] }) })
    expect(w.text()).toContain(t.checkAllMatched)
    expect(calls.some((c) => c.path === '/assistant/ingredients')).toBe(false)
  })

  it('approves all existing suggestions, keeps new ones, drops no-ops and none', async () => {
    const { w, calls } = await show({
      'GET /assistant/ingredient-candidates': () => ({ candidates: [cand('czerwona cebula', { currentProductId: 'p0' }), cand('tofu'), cand('sól'), cand('mleko', { currentProductId: 'p0' })] }),
      'POST /assistant/ingredients': () => ({
        suggestions: [
          { name: 'czerwona cebula', kind: 'existing', productId: 'p9', productName: 'Cebula czerwona', reason: 'exact' },
          { name: 'tofu', kind: 'new', newProduct: { name: 'Tofu', per100g: {}, pieceGrams: null, densityGPerMl: null, aliases: [] }, reason: 'model' },
          { name: 'sól', kind: 'none', reason: 'model' },
          { name: 'mleko', kind: 'existing', productId: 'p0', productName: 'Cebula', reason: 'model' },
        ],
      }),
      'POST /products/p9/assign': () => ({ updated: 2, aliasAdded: true, aliasConflict: '' }),
    })
    expect(w.text()).toContain('teraz: Cebula')
    expect(w.text()).toContain('Zupa, Leczo')
    expect(w.text()).toContain('1 składnik bez propozycji')
    buttonByText(t.checkApproveAll).click()
    for (let i = 0; i < 4; i++) await flush()
    const assigns = calls.filter((c) => c.path.endsWith('/assign'))
    expect(assigns).toHaveLength(1)
    expect(assigns[0].body).toEqual({ name: 'czerwona cebula', recipeIds: ['r1', 'r2'] })
    expect(useNotice().text).toBe('Przypisano czerwona cebula w 2 przepisach')
    expect(w.text()).toContain('tofu')
    expect(w.text()).not.toContain('mleko') // already assigned to the suggested product
    expect(w.text()).not.toContain('czerwona cebula')
  })

  it('stops sending batches after Przerwij', async () => {
    const many = Array.from({ length: 26 }, (_, i) => cand(`skladnik${i}`))
    let batches = 0
    const { calls } = await show({
      'GET /assistant/ingredient-candidates': () => ({ candidates: many }),
      'POST /assistant/ingredients': () => {
        batches++
        buttonByText(t.checkStop).click()
        return { suggestions: [] }
      },
    })
    expect(batches).toBe(1)
    expect(calls.filter((c) => c.path === '/assistant/ingredients')).toHaveLength(1)
  })
})
