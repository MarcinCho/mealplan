import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import MembersView from './MembersView.vue'
import { buttonByText, fakeApi, flush, mountWith, type Routes } from '../test/mount'
import { t } from '../messages'
import { useSession } from '../stores/session'
import { useMealPrepStore } from '../stores/mealprep'

const api = vi.hoisted(() => ({ current: (async () => ({})) as (m: string, p: string, b?: unknown) => Promise<unknown> }))
vi.mock('../api/client', () => ({ api: (m: string, p: string, b?: unknown) => api.current(m, p, b), ApiError: class extends Error {} }))

const members = [
  { id: 'u1', login: 'marc', displayName: 'Marc', role: 'admin' },
  { id: 'u2', login: 'ola', displayName: 'Ola', role: 'member' },
]

function routes(extra: Routes = {}): Routes {
  return {
    'GET /household/members': () => members,
    'GET /auth/invites': () => [],
    'GET /household/settings': () => ({ portionsPerMeal: 3 }),
    'PUT /household/settings': (b) => b,
    'DELETE /household/members/u2': () => null,
    ...extra,
  }
}

async function show(role: 'admin' | 'member', r: Routes = routes()) {
  const f = fakeApi(r)
  api.current = f.fn
  const pinia = createPinia()
  setActivePinia(pinia)
  useSession().$patch({ me: { user: { id: role === 'admin' ? 'u1' : 'u2', login: 'x', displayName: 'X' }, household: { id: 'h1', name: 'Dom' }, role } as never, loaded: true })
  const w = mountWith(MembersView, {}, pinia)
  for (let i = 0; i < 4; i++) await flush()
  return { w, calls: f.calls }
}

async function typeInto(input: HTMLInputElement, value: string) {
  input.value = value
  input.dispatchEvent(new Event('input'))
  await flush()
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('MembersView', () => {
  it('loads the household portions and saves a new value', async () => {
    const { w, calls } = await show('admin')
    const input = w.find('input[type="number"]').element as HTMLInputElement
    expect(input.value).toBe('3')
    await typeInto(input, '4')
    buttonByText(t.save).click()
    await flush()
    await flush()
    expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ portionsPerMeal: 4 })
    expect(useMealPrepStore().portionsPerMeal).toBe(4)
    expect(buttonByText(t.settingSaved)).toBeTruthy()
  })

  it('refuses portions outside 1-20', async () => {
    const { w, calls } = await show('admin')
    const input = w.find('input[type="number"]').element as HTMLInputElement
    for (const v of ['0', '21', '2.5']) {
      await typeInto(input, v)
      expect(buttonByText(t.save).disabled).toBe(true)
    }
    expect(calls.some((c) => c.method === 'PUT')).toBe(false)
  })

  it('asks for a second click before removing a member', async () => {
    const { calls } = await show('admin')
    buttonByText(t.removeMember).click()
    await flush()
    expect(calls.some((c) => c.method === 'DELETE')).toBe(false)
    buttonByText(t.confirmRemoveMember).click()
    await flush()
    expect(calls.find((c) => c.method === 'DELETE')?.path).toBe('/household/members/u2')
  })

  it('shows roles read-only to a member and no invites', async () => {
    const { w, calls } = await show('member')
    expect(w.text()).toContain(t.admin)
    expect(w.text()).not.toContain(t.invites)
    expect(() => buttonByText(t.removeMember)).toThrow()
    expect(calls.some((c) => c.path === '/auth/invites')).toBe(false)
  })
})
