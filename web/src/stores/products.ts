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

// A cleared v-text-field sets its model to null; treat that as an empty search.
export function normalizeQuery(q: string | null | undefined): string {
  return (q ?? '').trim()
}

let inflight: Promise<void> | null = null

export const useProducts = defineStore('products', {
  state: () => ({ items: [] as Product[], loaded: false, catalog: [] as Product[], catalogLoaded: false }),
  actions: {
    // The whole household list, shared by every picker: one request no matter how many ask.
    ensureLoaded(force = false): Promise<void> {
      if (inflight) return inflight
      if (this.catalogLoaded && !force) return Promise.resolve()
      inflight = api<Product[]>('GET', '/products')
        .then((list) => {
          this.catalog = list
          this.catalogLoaded = true
        })
        .finally(() => {
          inflight = null
        })
      return inflight
    },
    // Finds a product by id for display; refreshes the cache once when it is not there.
    async resolve(id: string | null | undefined): Promise<Product | null> {
      if (!id) return null
      await this.ensureLoaded()
      let p = this.catalog.find((x) => x.id === id)
      if (!p) {
        await this.ensureLoaded(true)
        p = this.catalog.find((x) => x.id === id)
      }
      return p ?? null
    },
    addToCatalog(p: Product) {
      this.catalog = [p, ...this.catalog.filter((x) => x.id !== p.id)]
    },
    async load(q?: string | null) {
      const n = normalizeQuery(q)
      const query = n ? `?q=${encodeURIComponent(n)}` : ''
      this.items = await api<Product[]>('GET', '/products' + query)
      this.loaded = true
    },
    async save(p: Product): Promise<Product> {
      const body = {
        name: (p.name ?? '').trim(),
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
      const saved = p.id
        ? await api<Product>('PUT', `/products/${encodeURIComponent(p.id)}`, body)
        : await api<Product>('POST', '/products', body)
      // Keep the shared list other views read in step with the change.
      if (this.catalogLoaded) {
        const i = this.catalog.findIndex((x) => x.id === saved.id)
        if (i >= 0) this.catalog.splice(i, 1, saved)
        else this.catalog = [saved, ...this.catalog]
      }
      return saved
    },
    async remove(id: string) {
      await api('DELETE', `/products/${encodeURIComponent(id)}`)
      this.items = this.items.filter((p) => p.id !== id)
      this.catalog = this.catalog.filter((p) => p.id !== id)
    },
  },
})
