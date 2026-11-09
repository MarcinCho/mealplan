import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { Recipe } from '../api/types'

export function emptyRecipe(): Recipe {
  return { id: '', title: '', baseServings: 4, sourceUrl: '', tags: [], ingredients: [], steps: [] }
}

export function formatQuantity(q: number | null, unit: string): string {
  if (q === null) return ''
  const n = q.toLocaleString('pl-PL', { maximumFractionDigits: 3 })
  return unit ? `${n} ${unit}` : n
}

// v-model.number yields '' when a field is cleared; the API expects null. 0 is a real value.
const num = (v: unknown): number | null => (v === '' || v === undefined || v === null ? null : Number(v))

const blankIngredient = (i: Recipe['ingredients'][number]): boolean =>
  !i.name.trim() && num(i.quantity) === null && !i.unit.trim() && !(i.note ?? '').trim() && !i.productId

export const useRecipes = defineStore('recipes', {
  state: () => ({ items: [] as Recipe[], loaded: false }),
  actions: {
    async load() {
      this.items = await api<Recipe[]>('GET', '/recipes')
      this.loaded = true
    },
    async get(id: string): Promise<Recipe> {
      return api<Recipe>('GET', `/recipes/${encodeURIComponent(id)}`)
    },
    async save(r: Recipe): Promise<Recipe> {
      const body = {
        title: r.title,
        baseServings: r.baseServings,
        sourceUrl: r.sourceUrl,
        tags: r.tags,
        // v-model.number yields '' when the field is cleared; the API expects null.
        // Only schema fields are sent: imported rows carry extra hints the server rejects.
        // A row left completely empty (e.g. "Dodaj składnik" clicked once too often) is dropped, not rejected.
        ingredients: r.ingredients.filter((i) => !blankIngredient(i)).map((i) => ({
          name: i.name,
          quantity: (i.quantity as unknown) === '' ? null : i.quantity,
          unit: i.unit,
          note: (i.note ?? '').trim(),
          // PUT replaces the whole recipe: an omitted productId would clear the stored choice.
          productId: i.productId ? i.productId : null,
        })),
        steps: r.steps.filter((s) => s.trim() !== ''),
        nutrition: {
          kcal: num(r.nutrition?.kcal),
          protein: num(r.nutrition?.protein),
          carbs: num(r.nutrition?.carbs),
          fat: num(r.nutrition?.fat),
        },
      }
      const saved = r.id
        ? await api<Recipe>('PUT', `/recipes/${encodeURIComponent(r.id)}`, body)
        : await api<Recipe>('POST', '/recipes', body)
      this.loaded = false
      return saved
    },
    async remove(id: string) {
      await api('DELETE', `/recipes/${encodeURIComponent(id)}`)
      this.items = this.items.filter((r) => r.id !== id)
    },
  },
})
