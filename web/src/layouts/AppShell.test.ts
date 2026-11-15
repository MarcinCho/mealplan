import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { defineComponent, h } from 'vue'
import { VApp } from 'vuetify/components'
import AppShell from './AppShell.vue'
import { flush, mountWith } from '../test/mount'
import { t } from '../messages'
import { useSession } from '../stores/session'

vi.mock('../api/client', () => ({ api: vi.fn(async () => ({})), ApiError: class extends Error {} }))

const Stub = defineComponent({ render: () => null })
// Vuetify layout components need a v-app around them.
const Shell = defineComponent({ render: () => h(VApp, () => h(AppShell)) })

afterEach(() => {
  document.body.innerHTML = ''
})

describe('AppShell', () => {
  it('closes the account menu when one of its links opens a page', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useSession().$patch({ me: { user: { id: 'u1', login: 'x', displayName: 'Ola' }, household: { id: 'h1', name: 'Dom' }, role: 'admin' } as never, loaded: true })
    const names = ['home', 'plan', 'prep', 'recipes', 'shopping', 'pantry', 'account', 'products', 'members', 'household']
    const router = createRouter({ history: createMemoryHistory(), routes: names.map((name) => ({ path: name === 'home' ? '/' : `/${name}`, name, component: Stub })) })
    await router.push('/')
    await router.isReady()
    mountWith(Shell, { global: { plugins: [router] } }, pinia)
    await flush()
    document.querySelector<HTMLButtonElement>(`button[title="${t.account}"]`)!.click()
    for (let i = 0; i < 3; i++) await flush()
    const link = [...document.querySelectorAll<HTMLElement>('.v-list-item')].find((e) => e.textContent?.includes(t.householdSettings))
    expect(link).toBeTruthy()
    link!.click()
    for (let i = 0; i < 4; i++) await flush()
    expect(router.currentRoute.value.name).toBe('household')
    expect(document.querySelector('.v-overlay--active .v-list')).toBeNull()
  })
})
