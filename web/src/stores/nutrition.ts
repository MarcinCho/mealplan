import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { DaySummary, NutritionSettings } from '../api/types'

export interface LogInput {
  date: string
  productId: string
  quantity: number
  unit: 'g' | 'szt'
}

const num = (v: unknown): number | null => (v === '' || v === undefined || v === null ? null : Number(v))

export const useNutrition = defineStore('nutrition', {
  state: () => ({ settings: null as NutritionSettings | null, days: {} as Record<string, DaySummary> }),
  actions: {
    async loadSettings() {
      this.settings = await api<NutritionSettings>('GET', '/me/nutrition')
    },
    async saveSettings(s: NutritionSettings) {
      const body = {
        enabled: s.enabled,
        goal: { kcal: num(s.goal.kcal), protein: num(s.goal.protein), carbs: num(s.goal.carbs), fat: num(s.goal.fat) },
      }
      this.settings = await api<NutritionSettings>('PUT', '/me/nutrition', body)
    },
    async loadDays(from: string, to: string) {
      const res = await api<{ enabled: boolean; days: DaySummary[] }>(
        'GET',
        `/me/nutrition/days?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
      )
      const next: Record<string, DaySummary> = { ...this.days }
      for (const d of res.days) next[d.date] = d
      this.days = next
    },
    async addLog(input: LogInput) {
      await api('POST', '/me/food-log', input)
    },
    async removeLog(id: string) {
      await api('DELETE', `/me/food-log/${encodeURIComponent(id)}`)
    },
  },
})
