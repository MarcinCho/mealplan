import { defineStore } from 'pinia'
import { api } from '../api/client'
import { pantryErrorText } from '../messages'
import { createSerialSaver, type SerialSaver } from '../pantry'
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

export interface Patch {
  quantity?: number | null
  expiresOn?: string
  staple?: boolean
}

// Savers live outside the component so a remount never runs two for the same item.
const savers = new Map<string, SerialSaver<Patch>>()

export const usePantry = defineStore('pantry', {
  state: () => ({
    items: [] as PantryItem[],
    reviewedAt: null as string | null,
    loaded: false,
    // Optimistic values while saves are pending; absent means "show the server's value".
    shownQty: {} as Record<string, number | null>,
    shownExpiry: {} as Record<string, string>,
    shownStaple: {} as Record<string, boolean>,
    zeroId: null as string | null,
    saveError: '',
  }),
  actions: {
    clearShown(id: string) {
      delete this.shownQty[id]
      delete this.shownExpiry[id]
      delete this.shownStaple[id]
    },
    saverFor(id: string): SerialSaver<Patch> {
      let sv = savers.get(id)
      if (!sv) {
        sv = createSerialSaver<Patch>({
          delay: 400,
          merge: (a, b) => ({ ...a, ...b }),
          send: async (patch) => {
            const cur = this.items.find((x) => x.id === id)
            if (!cur) return
            const staple = patch.staple ?? cur.staple
            await this.update(id, {
              quantity: staple ? null : (patch.quantity ?? cur.quantity ?? 0),
              unit: cur.unit,
              expiresOn: patch.expiresOn ?? cur.expiresOn,
              staple,
            })
          },
          onIdle: () => {
            this.clearShown(id)
            const it = this.items.find((x) => x.id === id)
            if (it && !it.staple && it.quantity === 0) this.zeroId = id
            else if (this.zeroId === id) this.zeroId = null
          },
          onError: (e) => {
            this.clearShown(id) // roll back to what the server has
            this.saveError = pantryErrorText(e)
          },
        })
        savers.set(id, sv)
      }
      return sv
    },
    queue(id: string, patch: Patch) {
      this.saveError = ''
      if (patch.quantity !== undefined) this.shownQty[id] = patch.quantity
      if (patch.expiresOn !== undefined) this.shownExpiry[id] = patch.expiresOn
      if (patch.staple !== undefined) this.shownStaple[id] = patch.staple
      this.saverFor(id).set(patch)
    },
    // Drops any pending save for the item; call before deleting it.
    cancelSaves(id: string) {
      savers.get(id)?.cancel()
      savers.delete(id)
      this.clearShown(id)
      if (this.zeroId === id) this.zeroId = null
    },
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
      this.cancelSaves(id)
      await api('DELETE', `/pantry/${encodeURIComponent(id)}`)
      this.items = this.items.filter((x) => x.id !== id)
    },
    async markReviewed() {
      const res = await api<{ reviewedAt: string }>('POST', '/pantry/review')
      this.reviewedAt = res.reviewedAt
    },
  },
})
