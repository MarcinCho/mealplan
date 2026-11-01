import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { PlanEntry, Slot } from '../api/types'

export interface EntryInput {
  date: string
  slot: Slot
  recipeId?: string | null
  freeText?: string
  servings?: number
  cookUserId?: string | null
}

// Monotonic request counter: only the latest load may touch the state.
let seq = 0

export const usePlan = defineStore('plan', {
  state: () => ({ week: '', entries: [] as PlanEntry[], loading: false, loaded: false }),
  getters: {
    entriesFor: (s) => (date: string, slot: Slot) => s.entries.filter((e) => e.date === date && e.slot === slot),
  },
  actions: {
    async load(week: string) {
      const mine = ++seq
      this.week = week
      this.entries = []
      this.loaded = false
      this.loading = true
      try {
        const res = await api<{ week: string; entries: PlanEntry[] }>('GET', `/plan/${week}`)
        if (mine !== seq) return
        this.entries = res.entries ?? []
        this.loaded = true
      } finally {
        if (mine === seq) this.loading = false
      }
    },
    async add(input: EntryInput) {
      const week = this.week
      const e = await api<PlanEntry>('POST', `/plan/${week}/entries`, input)
      if (week === this.week) this.entries.push(e)
    },
    async edit(id: string, input: EntryInput) {
      const week = this.week
      const e = await api<PlanEntry>('PUT', `/plan/${week}/entries/${encodeURIComponent(id)}`, input)
      if (week === this.week) this.entries = this.entries.map((x) => (x.id === id ? e : x))
    },
    async remove(id: string) {
      const week = this.week
      await api('DELETE', `/plan/${week}/entries/${encodeURIComponent(id)}`)
      if (week === this.week) this.entries = this.entries.filter((x) => x.id !== id)
    },
    async copyFrom(fromWeek: string) {
      const week = this.week
      await api('POST', `/plan/${week}/copy`, { fromWeek })
      await this.load(week)
    },
  },
})
