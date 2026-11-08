import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { Product } from '../api/types'

export function emptyProduct(): Product {
  return {
    id: '',
    name: '',
    per100g: { kcal: null, protein: null, carbs: null, fat: null },
    pieceGrams: null,
    densityGPerMl: null,
    aliases: [],
  }
}

// v-model.number yields '' when a field is cleared; the API expects null.
const num = (v: unknown): number | null => (v === '' || v === undefined || v === null ? null : Number(v))

export const useProducts = defineStore('products', {
  state: () => ({ items: [] as Product[], loaded: false }),
  actions: {
    async load(q?: string) {
      const query = q ? `?q=${encodeURIComponent(q)}` : ''
      this.items = await api<Product[]>('GET', '/products' + query)
      this.loaded = true
    },
    async save(p: Product): Promise<Product> {
      const body = {
        name: p.name.trim(),
        per100g: {
          kcal: num(p.per100g.kcal),
          protein: num(p.per100g.protein),
          carbs: num(p.per100g.carbs),
          fat: num(p.per100g.fat),
        },
        pieceGrams: num(p.pieceGrams),
        densityGPerMl: num(p.densityGPerMl),
        aliases: p.aliases,
      }
      return p.id
        ? api<Product>('PUT', `/products/${encodeURIComponent(p.id)}`, body)
        : api<Product>('POST', '/products', body)
    },
    async remove(id: string) {
      await api('DELETE', `/products/${encodeURIComponent(id)}`)
      this.items = this.items.filter((p) => p.id !== id)
    },
  },
})
