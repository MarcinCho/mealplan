import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import ProductsView from './ProductsView.vue'
import { fakeApi, flush, mountWith } from '../test/mount'
import { t } from '../messages'

const api = vi.hoisted(() => ({ current: (async () => ({})) as (m: string, p: string, b?: unknown) => Promise<unknown> }))
vi.mock('../api/client', () => ({ api: (m: string, p: string, b?: unknown) => api.current(m, p, b), ApiError: class extends Error {} }))

async function show(configured: boolean) {
  const f = fakeApi({
    'GET /household/assistant': () => ({ configured, keyHint: 'abcd', model: '', defaultModel: 'm' }),
    'GET /products': () => [{ id: 'p1', name: 'Cebula', per100g: { kcal: null, protein: null, carbs: null, fat: null }, pieceGrams: null, densityGPerMl: null, aliases: [] }],
  })
  api.current = f.fn
  const pinia = createPinia()
  setActivePinia(pinia)
  const w = mountWith(ProductsView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } }, pinia)
  for (let i = 0; i < 6; i++) await flush()
  return w
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('ProductsView', () => {
  it('shows the check button only when the assistant is configured', async () => {
    expect((await show(true)).text()).toContain(t.checkProducts)
    document.body.innerHTML = ''
    expect((await show(false)).text()).not.toContain(t.checkProducts)
  })

  it('offers Scal z… in the row menu', async () => {
    const w = await show(false)
    ;(w.find(`[aria-label="${t.moreActions}"]`).element as HTMLElement).click()
    await flush()
    await flush()
    expect(document.body.textContent).toContain(t.mergeWith)
  })
})
