import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { defineComponent, h } from 'vue'
import { VApp } from 'vuetify/components'
import AppShell from './AppShell.vue'
import { flush, mountWith, setViewportWidth } from '../test/mount'
import { t } from '../messages'
import { useSession } from '../stores/session'

vi.mock('../api/client', () => ({ api: vi.fn(async () => ({})), ApiError: class extends Error {} }))

const Stub = defineComponent({ render: () => null })
// Vuetify layout components need a v-app around them.
const Shell = defineComponent({ render: () => h(VApp, () => h(AppShell)) })

const NAV = [t.navToday, t.navPlan, t.navPrep, t.navRecipes, t.navPantry, t.navShopping]

afterEach(() => {
  document.body.innerHTML = ''
  setViewportWidth(1024)
})

async function mountShell(path: string, width: number, householdName = 'Dom') {
  const pinia = createPinia()
  setActivePinia(pinia)
  useSession().$patch({ me: { user: { id: 'u1', login: 'x', displayName: 'Ola' }, household: { id: 'h1', name: householdName }, role: 'admin' } as never, loaded: true })
  const names = ['home', 'plan', 'prep', 'recipes', 'shopping', 'pantry', 'account', 'products', 'members', 'household']
  const router = createRouter({ history: createMemoryHistory(), routes: names.map((name) => ({ path: name === 'home' ? '/' : `/${name}`, name, component: Stub })) })
  await router.push(path)
  await router.isReady()
  setViewportWidth(width)
  mountWith(Shell, { global: { plugins: [router] } }, pinia)
  await flush()
  return router
}

const navs = () => document.querySelectorAll<HTMLElement>(`nav[aria-label="${t.navMain}"]`)
const texts = (nav: Element) => [...nav.querySelectorAll('a')].map((a) => a.textContent?.trim())

async function openAccountMenu() {
  document.querySelector<HTMLButtonElement>(`button[title="${t.account}"]`)!.click()
  for (let i = 0; i < 3; i++) await flush()
}

describe('AppShell', () => {
  it('shows six text links in the top bar on desktop', async () => {
    await mountShell('/', 1440)
    expect(navs()).toHaveLength(1)
    expect(texts(navs()[0])).toEqual(NAV)
    expect(document.querySelector('.v-navigation-drawer')).toBeNull()
  })

  it('shows six tabs at the bottom on a phone', async () => {
    await mountShell('/', 390)
    expect(navs()).toHaveLength(1)
    expect(navs()[0].closest('.v-bottom-navigation')).not.toBeNull()
    expect(texts(navs()[0])).toEqual(NAV)
  })

  it.each([1440, 390])('marks the active link with aria-current at %i px', async (width) => {
    for (const [path, title] of [['/plan', t.navPlan], ['/', t.navToday], ['/recipes', t.navRecipes]]) {
      document.body.innerHTML = ''
      await mountShell(path, width)
      const active = document.querySelectorAll('a[aria-current="page"]')
      expect(active).toHaveLength(1)
      expect(active[0].textContent?.trim()).toBe(title)
    }
  })

  it('shows the household name in the bar on wide screens', async () => {
    await mountShell('/', 1440)
    const name = document.querySelector('.v-app-bar .household-name')
    expect(name?.textContent?.trim()).toBe('Dom')
    expect(name?.getAttribute('title')).toBe('Dom')
    await openAccountMenu()
    expect(document.querySelector('.v-overlay--active .household-name')).toBeNull()
  })

  it.each([1000, 390])('moves the household name into the account menu at %i px', async (width) => {
    await mountShell('/', width)
    expect(document.querySelector('.v-app-bar .household-name')).toBeNull()
    await openAccountMenu()
    expect(document.querySelector('.v-overlay--active .household-name')?.textContent?.trim()).toBe('Dom')
  })

  it('keeps a long household name whole in the title', async () => {
    await mountShell('/', 1440, 'D'.repeat(100))
    expect(document.querySelector('.household-name')?.getAttribute('title')).toHaveLength(100)
  })

  it('closes the account menu when one of its links opens a page', async () => {
    const router = await mountShell('/', 1024)
    await openAccountMenu()
    const link = [...document.querySelectorAll<HTMLElement>('.v-list-item')].find((e) => e.textContent?.includes(t.householdSettings))
    expect(link).toBeTruthy()
    link!.click()
    for (let i = 0; i < 4; i++) await flush()
    expect(router.currentRoute.value.name).toBe('household')
    expect(document.querySelector('.v-overlay--active .v-list')).toBeNull()
  })
})
