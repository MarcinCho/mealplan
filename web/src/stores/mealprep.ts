import { defineStore } from 'pinia'
import { api, ApiError } from '../api/client'
import { prepErrorText } from '../messages'
import { isoDate } from '../weeks'
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
    // write runs a change and refreshes afterwards. A pantry_failed error means the change itself was
    // saved, so the lists are refreshed then too before the error is passed on.
    async write<T>(req: Promise<T>): Promise<T> {
      try {
        const res = await req
        await this.refresh()
        return res
      } catch (e) {
        if (e instanceof ApiError && e.code === 'pantry_failed') await this.refresh()
        throw e
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
      const s = await this.write(api<PrepSession>('POST', '/prep/sessions', input))
      return s
    },
    async updateSession(id: string, input: { date: string; name: string; week: string; cooked: boolean }) {
      await this.write(api<PrepSession>('PUT', `/prep/sessions/${enc(id)}`, input))
    },
    async removeSession(id: string) {
      await this.write(api('DELETE', `/prep/sessions/${enc(id)}`))
    },
    async addDish(sessionId: string, recipeId: string, portions: number) {
      await this.write(api<PrepDish>('POST', `/prep/sessions/${enc(sessionId)}/dishes`, { recipeId, portions }))
    },
    async updateDish(id: string, portions: number, writtenOff: number) {
      await this.write(api<PrepDish>('PUT', `/prep/dishes/${enc(id)}`, { portions, writtenOff }))
    },
    async removeDish(id: string) {
      await this.write(api('DELETE', `/prep/dishes/${enc(id)}`))
    },
    async freeze(dishId: string, portions: number, frozenOn: string = isoDate(new Date())) {
      await this.write(api<FreezerItem>('POST', `/prep/dishes/${enc(dishId)}/freeze`, { portions, frozenOn }))
    },
    async addFreezer(input: FreezerInput) {
      await this.write(api<FreezerItem>('POST', '/freezer', input))
    },
    async updateFreezer(id: string, input: FreezerInput) {
      await this.write(api<FreezerItem>('PUT', `/freezer/${enc(id)}`, input))
    },
    async removeFreezer(id: string) {
      await this.write(api('DELETE', `/freezer/${enc(id)}`))
    },
  },
})
