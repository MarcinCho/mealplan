import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import AssistantSettings from './AssistantSettings.vue'
import { buttonByText, fakeApi, flush, mountWith } from '../test/mount'
import { t } from '../messages'
import { useSession } from '../stores/session'

const api = vi.hoisted(() => ({ current: (async () => ({})) as (m: string, p: string, b?: unknown) => Promise<unknown> }))
vi.mock('../api/client', () => ({ api: (m: string, p: string, b?: unknown) => api.current(m, p, b), ApiError: class extends Error {} }))

const settings = { configured: true, keyHint: 'abcd', model: 'vendor/model', defaultModel: 'vendor/default' }

async function show(role: 'admin' | 'member') {
  const f = fakeApi({
    'GET /household/assistant': () => settings,
    'PUT /household/assistant': () => settings,
  })
  api.current = f.fn
  const pinia = createPinia()
  setActivePinia(pinia)
  useSession().$patch({ me: { user: { id: 'u1', login: 'x', displayName: 'X' }, household: { id: 'h1', name: 'Dom' }, role } as never, loaded: true })
  const w = mountWith(AssistantSettings, {}, pinia)
  for (let i = 0; i < 3; i++) await flush()
  return { w, calls: f.calls }
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('AssistantSettings', () => {
  it('shows the admin a password key field, empty, and a save button', async () => {
    const { w } = await show('admin')
    const input = w.find('input[type="password"]')
    expect(input.exists()).toBe(true)
    expect((input.element as HTMLInputElement).value).toBe('')
    expect(input.attributes('autocomplete')).toBe('off')
    expect(w.text()).toContain('Klucz ustawiony, kończy się na …abcd')
    expect(buttonByText(t.save)).toBeTruthy()
  })

  it('shows a member only the key hint, without a field', async () => {
    const { w } = await show('member')
    expect(w.text()).toContain('Klucz ustawiony, kończy się na …abcd')
    expect(w.find('input').exists()).toBe(false)
    expect(() => buttonByText(t.save)).toThrow()
  })

  it('saves without a typed key by sending no key', async () => {
    const { calls } = await show('admin')
    buttonByText(t.save).click()
    await flush()
    await flush()
    const put = calls.find((c) => c.method === 'PUT')
    expect(put?.body).toEqual({ model: 'vendor/model' })
    expect(put?.body).not.toHaveProperty('key')
  })

  it('sends the typed key and clears the field afterwards', async () => {
    const { w, calls } = await show('admin')
    const input = w.find('input[type="password"]').element as HTMLInputElement
    input.value = 'sk-new'
    input.dispatchEvent(new Event('input'))
    await flush()
    buttonByText(t.save).click()
    await flush()
    await flush()
    expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ key: 'sk-new', model: 'vendor/model' })
    expect(input.value).toBe('')
  })
})
