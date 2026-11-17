import { mount, type ComponentMountingOptions } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import type { Component } from 'vue'

// mountWith mounts a component with Vuetify and a fresh Pinia, attached to the document so that
// dialogs and menus (teleported to body) can be found with document queries.
export function mountWith<C extends Component>(comp: C, options: ComponentMountingOptions<C> = {}, pinia: Pinia = createPinia()) {
  setActivePinia(pinia)
  const vuetify = createVuetify({ components, directives })
  const el = document.createElement('div')
  document.body.appendChild(el)
  const global = options.global ?? {}
  return mount(comp, { attachTo: el, ...options, global: { ...global, plugins: [vuetify, pinia, ...(global.plugins ?? [])] } } as ComponentMountingOptions<C>)
}

export const flush = () => new Promise((r) => setTimeout(r, 0))

export function buttonByText(text: string): HTMLButtonElement {
  const b = [...document.querySelectorAll('button')].find((x) => x.textContent?.trim() === text)
  if (!b) throw new Error(`no button "${text}"`)
  return b as HTMLButtonElement
}

// A fake API: each handler key is "METHOD /path"; unknown calls fail like a 404.
export type Routes = Record<string, (body?: unknown) => unknown>
export function fakeApi(routes: Routes) {
  const calls: { method: string; path: string; body?: unknown }[] = []
  const fn = async (method: string, path: string, body?: unknown) => {
    calls.push({ method, path, body })
    const h = routes[`${method} ${path}`]
    if (!h) throw new Error(`unexpected ${method} ${path}`)
    return h(body)
  }
  return { fn, calls }
}

// setViewportWidth sets the width Vuetify's useDisplay sees; mountWith creates a fresh Vuetify per mount,
// so call it before mounting. Files using it restore setViewportWidth(1024) (the jsdom default) in afterEach.
export function setViewportWidth(px: number): void {
  Object.defineProperty(window, 'innerWidth', { value: px, configurable: true, writable: true })
}

// clickable returns the innermost button, link or tabindex element whose text contains `text`, so tests
// do not depend on the markup that renders it (chip, list item, plain button).
export function clickable(text: string): HTMLElement {
  const hits = [...document.querySelectorAll<HTMLElement>('button, a, [tabindex]')].filter((x) => x.textContent?.includes(text))
  const inner = hits.filter((x) => !hits.some((o) => o !== x && x.contains(o)))
  if (!inner.length) throw new Error(`nothing clickable contains "${text}"`)
  return inner[0]
}

export const openDialog = (): HTMLElement | null => document.querySelector<HTMLElement>('.v-overlay--active')
