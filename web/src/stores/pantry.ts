import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { PantryItem, PantryList } from '../api/types'

export interface PantryInput {
  productId?: string
  name?: string
  quantity: number | null
  unit: string
  expiresOn: string
  staple: boolean
}
export type PantryUpdate = Omit<PantryInput, 'productId' | 'name'>

// v-model.number yields '' for a cleared field.
const qty = (v: unknown): number | null => (v === '' || v === undefined || v === null ? null : Number(v))

export const usePantry = defineStore('pantry', {
  state: () => ({ items: [] as PantryItem[], reviewedAt: null as string | null, loaded: false }),
  actions: {
    async load() {
      const res = await api<PantryList>('GET', '/pantry')
      this.items = res.items
      this.reviewedAt = res.reviewedAt
      this.loaded = true
    },
    // The server may merge into an existing row and may rename (alias → product), so show what it returns.
    async add(input: PantryInput): Promise<PantryItem> {
      const it = await api<PantryItem>('POST', '/pantry', { ...input, quantity: qty(input.quantity) })
      const i = this.items.findIndex((x) => x.id === it.id)
      if (i >= 0) this.items.splice(i, 1, it)
      else this.items.push(it)
      return it
    },
    async update(id: string, input: PantryUpdate): Promise<PantryItem> {
      const it = await api<PantryItem>('PUT', `/pantry/${encodeURIComponent(id)}`, { ...input, quantity: qty(input.quantity) })
      this.items = this.items.map((x) => (x.id === id ? it : x))
      return it
    },
    async remove(id: string) {
      await api('DELETE', `/pantry/${encodeURIComponent(id)}`)
      this.items = this.items.filter((x) => x.id !== id)
    },
    async markReviewed() {
      const res = await api<{ reviewedAt: string }>('POST', '/pantry/review')
      this.reviewedAt = res.reviewedAt
    },
  },
})
