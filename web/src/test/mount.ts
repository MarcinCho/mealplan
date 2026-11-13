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
