import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import HouseholdView from './HouseholdView.vue'
import { fakeApi, flush, mountWith, type Routes } from '../test/mount'
import { t } from '../messages'
import { useSession } from '../stores/session'
import { useMealPrepStore } from '../stores/mealprep'

const api = vi.hoisted(() => ({ current: (async () => ({})) as (m: string, p: string, b?: unknown) => Promise<unknown> }))
vi.mock('../api/client', () => ({ api: (m: string, p: string, b?: unknown) => api.current(m, p, b), ApiError: class extends Error {} }))

function routes(extra: Routes = {}): Routes {
  return {
    'GET /household/settings': () => ({ name: 'Dom', portionsPerMeal: 3 }),
    'PUT /household/settings': (b) => ({ name: 'Dom', portionsPerMeal: 3, ...(b as object) }),
    'GET /household/assistant': () => ({ configured: false, keyHint: '', model: '', defaultModel: 'vendor/m' }),
    'GET /auth/me': () => ({ user: { id: 'u1', login: 'x', displayName: 'X' }, household: { id: 'h1', name: 'Dom nad jeziorem' }, role: 'admin', csrfToken: 'c' }),
    ...extra,
  }
}

async function show(role: 'admin' | 'member', r: Routes = routes()) {
  const f = fakeApi(r)
  api.current = f.fn
  const pinia = createPinia()
  setActivePinia(pinia)
  useSession().$patch({ me: { user: { id: 'u1', login: 'x', displayName: 'X' }, household: { id: 'h1', name: 'Dom' }, role } as never, loaded: true })
  const w = mountWith(HouseholdView, {}, pinia)
  for (let i = 0; i < 4; i++) await flush()
  return { w, calls: f.calls }
}

async function typeInto(input: HTMLInputElement, value: string) {
  input.value = value
  input.dispatchEvent(new Event('input'))
  await flush()
}

const portionsInput = (w: Awaited<ReturnType<typeof show>>['w']) => w.find('input[type="number"]').element as HTMLInputElement
const nameInput = (w: Awaited<ReturnType<typeof show>>['w']) => w.find('input[maxlength="100"]').element as HTMLInputElement

// The page has several "Zapisz" buttons: name (0, admin only), portions, assistant.
function buttonByText(text: string, i: number): HTMLButtonElement {
  const b = [...document.querySelectorAll('button')].filter((x) => x.textContent?.trim() === text)[i]
  if (!b) throw new Error(`no button "${text}" #${i}`)
  return b as HTMLButtonElement
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('HouseholdView', () => {
  it('loads the household portions and saves a new value', async () => {
    const { w, calls } = await show('admin')
    expect(portionsInput(w).value).toBe('3')
    await typeInto(portionsInput(w), '4')
    buttonByText(t.save, 1).click()
    await flush()
    await flush()
    expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ portionsPerMeal: 4 })
    expect(useMealPrepStore().portionsPerMeal).toBe(4)
  })

  it('refuses portions outside 1-20', async () => {
    const { w, calls } = await show('admin')
    for (const v of ['0', '21', '2.5']) {
      await typeInto(portionsInput(w), v)
      expect(buttonByText(t.save, 1).disabled).toBe(true)
    }
    expect(calls.some((c) => c.method === 'PUT')).toBe(false)
  })

  it('lets an administrator rename the household and refreshes the session', async () => {
    const { w, calls } = await show('admin')
    expect(nameInput(w).value).toBe('Dom')
    await typeInto(nameInput(w), 'Dom nad jeziorem')
    buttonByText(t.save, 0).click()
    for (let i = 0; i < 3; i++) await flush()
    expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ name: 'Dom nad jeziorem' })
    expect(calls.some((c) => c.path === '/auth/me')).toBe(true)
    expect(useSession().me?.household.name).toBe('Dom nad jeziorem')
  })

  it('refuses an empty name', async () => {
    const { w } = await show('admin')
    await typeInto(nameInput(w), '   ')
    expect(buttonByText(t.save, 0).disabled).toBe(true)
  })

  it('shows the name read-only to a member, who still sets portions', async () => {
    const { w } = await show('member')
    expect(w.find('input[maxlength="100"]').exists()).toBe(false)
    expect(w.find('[data-test="household-name"]').text()).toBe('Dom')
    expect(w.text()).toContain(t.householdNameAdminOnly)
    expect(portionsInput(w).value).toBe('3')
  })
})
