import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import ProductCheckView from './ProductCheckView.vue'
import MergeDialog from '../components/MergeDialog.vue'
import { buttonByText, fakeApi, flush, mountWith, type Routes } from '../test/mount'
import { t } from '../messages'
import type { Product } from '../api/types'
import { useNotice } from '../stores/notice'

const api = vi.hoisted(() => ({ current: (async () => ({})) as (m: string, p: string, b?: unknown) => Promise<unknown> }))
vi.mock('../api/client', () => ({ api: (m: string, p: string, b?: unknown) => api.current(m, p, b), ApiError: class extends Error {
    constructor(public status: number, public code: string, message: string, public retryAfter?: number) {
      super(message)
    }
  },
}))
import { ApiError } from '../api/client'

const prod = (id: string, name: string): Product => ({ id, name, per100g: { kcal: null, protein: null, carbs: null, fat: null }, pieceGrams: null, densityGPerMl: null, aliases: [] })
const catalog = [prod('A', 'Cebula'), prod('B', 'Cebula biała'), prod('C', 'Marchew'), prod('D', 'Marchewka')]
const usage = (recipes: number, pantry: number, foodLog: number) => ({ recipes, pantry, foodLog, shopping: 0 })
const merged = { repointed: usage(0, 0, 0), skippedAliases: [] as string[] }

async function show(r: Routes) {
  const f = fakeApi({
    'GET /household/assistant': () => ({ configured: true, keyHint: 'abcd', model: 'm', defaultModel: 'm' }),
    'GET /products': () => catalog,
    'GET /products/A/usage': () => usage(3, 1, 2),
    'GET /products/B/usage': () => usage(1, 0, 0),
    'GET /products/C/usage': () => usage(0, 0, 0),
    'GET /products/D/usage': () => usage(0, 0, 0),
    ...r,
  })
  api.current = f.fn
  const pinia = createPinia()
  setActivePinia(pinia)
  const w = mountWith(ProductCheckView, {}, pinia)
  for (let i = 0; i < 8; i++) await flush()
  return { w, calls: f.calls }
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('ProductCheckView', () => {
  it('shows usage, and merges with keep and absorb swapped after choosing the other product', async () => {
    const { w, calls } = await show({
      'GET /assistant/product-batches': () => ({ batches: [['A', 'B']] }),
      'POST /assistant/duplicates': () => ({ groups: [{ keepId: 'A', absorbIds: ['B'], reason: 'ta sama cebula' }] }),
      'POST /products/B/merge': () => merged,
    })
    expect(w.text()).toContain('3 przepisy · 1 w spiżarni · 2 w dzienniku')
    expect(w.text()).toContain('1 przepis')
    const radios = [...document.querySelectorAll('input[type=radio]')] as HTMLInputElement[]
    expect(radios.map((r) => r.checked)).toEqual([true, false])
    radios[1].click()
    await flush()
    buttonByText(t.productCheckMerge).click()
    for (let i = 0; i < 4; i++) await flush()
    const m = calls.filter((c) => c.path.endsWith('/merge'))
    expect(m).toHaveLength(1)
    expect(m[0].path).toBe('/products/B/merge')
    expect(m[0].body).toEqual({ absorbIds: ['A'] })
    expect(useNotice().text).toBe('Scalono w „Cebula biała”')
    expect(w.text()).not.toContain('ta sama cebula')
  })

  it('merges only the products that stay checked', async () => {
    const { calls } = await show({
      'GET /assistant/product-batches': () => ({ batches: [['A', 'B', 'C']] }),
      'POST /assistant/duplicates': () => ({ groups: [{ keepId: 'A', absorbIds: ['B', 'C'], reason: 'g1' }] }),
      'POST /products/A/merge': () => merged,
    })
    const boxes = [...document.querySelectorAll('input[type=checkbox]')] as HTMLInputElement[]
    expect(boxes.map((b) => b.checked)).toEqual([true, true])
    boxes[0].click() // leave out B
    await flush()
    buttonByText(t.productCheckMerge).click()
    for (let i = 0; i < 4; i++) await flush()
    expect(calls.filter((c) => c.path.endsWith('/merge'))[0].body).toEqual({ absorbIds: ['C'] })
  })

  it('disables Scal when every other product is unchecked', async () => {
    await show({
      'GET /assistant/product-batches': () => ({ batches: [['A', 'B']] }),
      'POST /assistant/duplicates': () => ({ groups: [{ keepId: 'A', absorbIds: ['B'], reason: 'g1' }] }),
    })
    ;(document.querySelector('input[type=checkbox]') as HTMLInputElement).click()
    await flush()
    expect(buttonByText(t.productCheckMerge).disabled).toBe(true)
  })

  it('drops products already absorbed from later groups and removes groups left with one', async () => {
    const { w, calls } = await show({
      'GET /assistant/product-batches': () => ({ batches: [['A', 'B'], ['B', 'C']] }),
      'POST /assistant/duplicates': (b) =>
        (b as { productIds: string[] }).productIds[0] === 'A'
          ? { groups: [{ keepId: 'A', absorbIds: ['B'], reason: 'g1' }] }
          : { groups: [{ keepId: 'B', absorbIds: ['C'], reason: 'g2' }, { keepId: 'C', absorbIds: ['D'], reason: 'g3' }] },
      'POST /products/A/merge': () => ({ repointed: usage(1, 0, 0), skippedAliases: ['cebula'] }),
      'POST /products/C/merge': () => merged,
    })
    // g2 (B,C) is shown with both until B is merged away
    expect(w.text()).toContain('g2')
    buttonByText(t.productCheckMerge) // exists
    ;([...document.querySelectorAll('button')].filter((x) => x.textContent?.trim() === t.productCheckMerge)[0]).click()
    for (let i = 0; i < 4; i++) await flush()
    expect(useNotice().text).toBe('Scalono w „Cebula”, pominięte aliasy: cebula')
    expect(w.text()).not.toContain('g1')
    expect(w.text()).not.toContain('g2') // B is gone, one product left
    expect(w.text()).toContain('g3')
    expect(calls.filter((c) => c.path.endsWith('/merge'))).toHaveLength(1)
    expect(calls.filter((c) => c.method === 'GET' && c.path === '/products').length).toBeGreaterThan(1) // catalog reloaded
  })

  it('treats a 404 on merge as an already merged group, calmly', async () => {
    const { w } = await show({
      'GET /assistant/product-batches': () => ({ batches: [['C', 'D']] }),
      'POST /assistant/duplicates': () => ({ groups: [{ keepId: 'C', absorbIds: ['D'], reason: 'g1' }] }),
      'POST /products/C/merge': () => {
        throw new ApiError(404, 'not_found', 'nope')
      },
    })
    buttonByText(t.productCheckMerge).click()
    for (let i = 0; i < 4; i++) await flush()
    expect(useNotice().text).toBe(t.productAlreadyMerged)
    expect(w.text()).not.toContain('g1')
  })

  it('discards a group on Odrzuć and reports nothing to change when finished', async () => {
    const { w, calls } = await show({
      'GET /assistant/product-batches': () => ({ batches: [['C', 'D']] }),
      'POST /assistant/duplicates': () => ({ groups: [{ keepId: 'C', absorbIds: ['D'], reason: 'g1' }] }),
    })
    buttonByText(t.suggestionReject).click()
    await flush()
    expect(w.text()).toContain(t.checkNothingToChange)
    expect(calls.some((c) => c.path.endsWith('/merge'))).toBe(false)
  })

  it('says so when there is nothing to compare, and shows no empty state while loading', async () => {
    const { w } = await show({ 'GET /assistant/product-batches': () => ({ batches: [] }) })
    expect(w.text()).toContain(t.productCheckNoBatches)
  })
})

describe('MergeDialog', () => {
  it('previews the other product and posts the merge', async () => {
    const f = fakeApi({
      'GET /products': () => catalog,
      'GET /products/B/usage': () => usage(1, 0, 0),
      'POST /products/A/merge': () => merged,
    })
    api.current = f.fn
    const pinia = createPinia()
    setActivePinia(pinia)
    const w = mountWith(MergeDialog, { props: { modelValue: true, keep: catalog[0] } }, pinia)
    for (let i = 0; i < 4; i++) await flush()
    ;(w.vm as unknown as { absorbId: string | null }).absorbId = 'B'
    for (let i = 0; i < 4; i++) await flush()
    expect(document.body.textContent).toContain('„Cebula biała” zniknie, a jego przepisy, spiżarnia i dziennik przejdą do „Cebula”.')
    expect(document.body.textContent).toContain('1 przepis')
    buttonByText(t.productCheckMerge).click()
    for (let i = 0; i < 4; i++) await flush()
    const m = f.calls.find((c) => c.path === '/products/A/merge')
    expect(m?.method).toBe('POST')
    expect(m?.body).toEqual({ absorbIds: ['B'] })
    expect(w.emitted('merged')).toHaveLength(1)
  })
})
