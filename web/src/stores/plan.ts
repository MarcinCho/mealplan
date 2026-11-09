import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { Macros, PlanEntry, Slot } from '../api/types'
import { useMealPrepStore } from './mealprep'

export interface EntryInput {
  date: string
  slot: Slot
  recipeId?: string | null
  freeText?: string
  servings?: number
  cookUserId?: string | null
  done?: boolean
  kind?: 'meal' | 'prep' | 'out'
  prepDishId?: string | null
  freezerItemId?: string | null
  outValues?: Macros | null
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
    // `forWeek` adds to a week other than the loaded one (the meal prep dialog picks any date).
    async add(input: EntryInput, forWeek?: string) {
      const week = forWeek ?? this.week
      const e = await api<PlanEntry>('POST', `/plan/${week}/entries`, input)
      if (week === this.week) this.entries.push(e)
      if (input.kind === 'prep') await useMealPrepStore().refresh()
    },
    async edit(id: string, input: EntryInput) {
      const week = this.week
      const e = await api<PlanEntry>('PUT', `/plan/${week}/entries/${encodeURIComponent(id)}`, input)
      if (week === this.week) this.entries = this.entries.map((x) => (x.id === id ? e : x))
      if (input.kind === 'prep') await useMealPrepStore().refresh()
    },
    // PUT replaces the entry, so the toggle sends the whole entry back with only `done` changed.
    async setDone(entry: PlanEntry, done: boolean) {
      await this.edit(entry.id, {
        date: entry.date,
        slot: entry.slot,
        recipeId: entry.recipeId,
        freeText: entry.freeText,
        servings: entry.servings,
        cookUserId: entry.cookUserId,
        done,
        kind: entry.kind ?? 'meal',
        prepDishId: entry.prepDishId ?? null,
        freezerItemId: entry.freezerItemId ?? null,
        outValues: entry.outValues ?? null,
      })
    },
    async remove(id: string) {
      const week = this.week
      const wasPrep = this.entries.find((x) => x.id === id)?.kind === 'prep'
      await api('DELETE', `/plan/${week}/entries/${encodeURIComponent(id)}`)
      if (week === this.week) this.entries = this.entries.filter((x) => x.id !== id)
      if (wasPrep) await useMealPrepStore().refresh()
    },
    async copyFrom(fromWeek: string) {
      const week = this.week
      await api('POST', `/plan/${week}/copy`, { fromWeek })
      if (this.week === week) await this.load(week)
    },
  },
})
