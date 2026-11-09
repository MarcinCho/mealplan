import { defineStore } from 'pinia'
import { api } from '../api/client'
import { prepErrorText } from '../messages'
import type { FreezerItem, PrepDish, PrepSession } from '../api/types'

export interface FreezerInput {
  recipeId?: string | null
  label?: string
  portions: number
  frozenOn?: string
}

const enc = encodeURIComponent

export const useMealPrepStore = defineStore('mealprep', {
  state: () => ({
    sessions: [] as PrepSession[],
    freezer: [] as FreezerItem[],
    portionsPerMeal: 1,
    loaded: false,
    // Set when a mutation saved but the refresh failed; the data is saved, so callers must not retry the write.
    reloadError: '',
  }),
  actions: {
    async reload() {
      const [s, f] = await Promise.all([
        api<{ sessions: PrepSession[] }>('GET', '/prep/sessions'),
        api<{ items: FreezerItem[] }>('GET', '/freezer'),
      ])
      this.sessions = s.sessions ?? []
      this.freezer = f.items ?? []
    },
    // Refresh after a successful write; a failure is recorded, not thrown.
    async refresh() {
      try {
        await this.reload()
        this.reloadError = ''
      } catch (e) {
        this.reloadError = prepErrorText(e)
      }
    },
    async load() {
      const [, settings] = await Promise.all([this.reload(), api<{ portionsPerMeal: number }>('GET', '/household/settings')])
      this.portionsPerMeal = settings.portionsPerMeal
      this.loaded = true
    },
    async savePortionsPerMeal(n: number) {
      const res = await api<{ portionsPerMeal: number }>('PUT', '/household/settings', { portionsPerMeal: n })
      this.portionsPerMeal = res.portionsPerMeal
    },
    async createSession(input: { date: string; name: string; week?: string }) {
      const s = await api<PrepSession>('POST', '/prep/sessions', input)
      await this.refresh()
      return s
    },
    async updateSession(id: string, input: { date: string; name: string; week: string; cooked: boolean }) {
      await api<PrepSession>('PUT', `/prep/sessions/${enc(id)}`, input)
      await this.refresh()
    },
    async removeSession(id: string) {
      await api('DELETE', `/prep/sessions/${enc(id)}`)
      await this.refresh()
    },
    async addDish(sessionId: string, recipeId: string, portions: number) {
      await api<PrepDish>('POST', `/prep/sessions/${enc(sessionId)}/dishes`, { recipeId, portions })
      await this.refresh()
    },
    async updateDish(id: string, portions: number, writtenOff: number) {
      await api<PrepDish>('PUT', `/prep/dishes/${enc(id)}`, { portions, writtenOff })
      await this.refresh()
    },
    async removeDish(id: string) {
      await api('DELETE', `/prep/dishes/${enc(id)}`)
      await this.refresh()
    },
    async freeze(dishId: string, portions: number) {
      await api<FreezerItem>('POST', `/prep/dishes/${enc(dishId)}/freeze`, { portions })
      await this.refresh()
    },
    async addFreezer(input: FreezerInput) {
      await api<FreezerItem>('POST', '/freezer', input)
      await this.refresh()
    },
    async updateFreezer(id: string, input: FreezerInput) {
      await api<FreezerItem>('PUT', `/freezer/${enc(id)}`, input)
      await this.refresh()
    },
    async removeFreezer(id: string) {
      await api('DELETE', `/freezer/${enc(id)}`)
      await this.refresh()
    },
  },
})
