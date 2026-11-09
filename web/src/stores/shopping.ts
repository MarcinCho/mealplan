import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { ShoppingItem } from '../api/types'

interface ListResponse {
  week: string
  items: ShoppingItem[]
  previousWeekUndone?: number
}

export const isCovered = (i: ShoppingItem) => i.quantity === 0 && (i.haveQuantity ?? 0) > 0

// Week most recently requested by load/generate; results for any other week are stale and dropped.
let requested = ''

export const useShopping = defineStore('shopping', {
  state: () => ({ week: '', items: [] as ShoppingItem[], previousWeekUndone: 0 }),
  getters: {
    open: (s) => s.items.filter((i) => !i.checked && !isCovered(i)),
    done: (s) => s.items.filter((i) => i.checked && !isCovered(i)),
    covered: (s) => s.items.filter(isCovered),
  },
  actions: {
    async load(week: string) {
      requested = week
      const res = await api<ListResponse>('GET', `/shopping?week=${encodeURIComponent(week)}`)
      if (requested !== week) return
      this.week = res.week
      this.items = res.items
      this.previousWeekUndone = res.previousWeekUndone ?? 0
    },
    async generate(week: string, range?: { from: string; to: string }) {
      requested = week
      const res = await api<ListResponse>('POST', '/shopping/generate', { week, ...range })
      if (requested !== week) return
      this.week = res.week
      this.items = res.items
      this.previousWeekUndone = res.previousWeekUndone ?? 0
    },
    async addManual(name: string, quantity: number | null, unit: string) {
      const week = this.week
      const q = (quantity as unknown) === '' ? null : quantity
      const it = await api<ShoppingItem>('POST', '/shopping/items', { week, name, quantity: q, unit })
      if (this.week === week) this.items.push(it)
    },
    async setChecked(id: string, checked: boolean) {
      const it = this.items.find((i) => i.id === id)
      if (!it) return
      const before = it.checked
      it.checked = checked
      try {
        await api('PATCH', `/shopping/items/${encodeURIComponent(id)}`, { checked })
      } catch (e) {
        it.checked = before
        throw e
      }
    },
    async edit(id: string, input: { name: string; quantity: number | null; unit: string }) {
      const body = { ...input, quantity: (input.quantity as unknown) === '' ? null : input.quantity }
      await api('PATCH', `/shopping/items/${encodeURIComponent(id)}`, body)
      this.items = this.items.map((i) => (i.id === id ? { ...i, ...body } : i))
    },
    async moveToPantry(week: string): Promise<number> {
      const res = await api<{ moved: number }>('POST', '/shopping/move-to-pantry', { week })
      // Reload only if the user is still on the moved week; otherwise a late load would clobber the visible one.
      if (requested === week) await this.load(week)
      return res.moved
    },
    async remove(id: string) {
      await api('DELETE', `/shopping/items/${encodeURIComponent(id)}`)
      this.items = this.items.filter((i) => i.id !== id)
    },
  },
})
