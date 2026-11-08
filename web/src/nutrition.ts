import type { DaySummary, Macros } from './api/types'

const KEYS: (keyof Macros)[] = ['kcal', 'protein', 'carbs', 'fat']

const GRAMS: Record<string, string> = { protein: 'białka', carbs: 'węglowodanów', fat: 'tłuszczu' }
const ABOUT: Record<string, string> = { kcal: 'kaloriach', protein: 'białku', carbs: 'węglowodanach', fat: 'tłuszczach' }

export function formatNumber(v: number | null, unit: 'kcal' | 'g'): string {
  if (v === null || v === undefined) return '—'
  return v.toLocaleString('pl-PL', unit === 'kcal' ? { maximumFractionDigits: 0 } : { maximumFractionDigits: 1 })
}

function amount(k: keyof Macros, v: number): string {
  return k === 'kcal' ? `${formatNumber(v, 'kcal')} kcal` : `${formatNumber(v, 'g')} g ${GRAMS[k]}`
}

export function formatMacros(m: Macros): string {
  const parts = KEYS.filter((k) => m[k] !== null && m[k] !== undefined).map((k) => amount(k, m[k] as number))
  return parts.length ? parts.join(' · ') : '—'
}

export function formatRemaining(d: DaySummary): string {
  const precise: string[] = []
  const atLeast: string[] = []
  const other: string[] = []
  let anyGoal = false
  for (const k of KEYS) {
    const goal = d.goal[k]
    if (goal === null || goal === undefined) continue
    anyGoal = true
    const total = d.total[k]
    if (total === null || total === undefined) {
      other.push(`brak danych o ${ABOUT[k]}`)
    } else if (d.incomplete[k]) {
      if (total < goal) atLeast.push(amount(k, goal - total))
      else other.push(`dane o ${ABOUT[k]} niepełne`)
    } else if (total < goal) {
      precise.push(amount(k, goal - total))
    }
  }
  if (!anyGoal) return 'brak celu'
  const out: string[] = []
  if (precise.length) out.push(`brakuje ${precise.join(' · ')}`)
  if (atLeast.length) out.push(`brakuje co najmniej ${atLeast.join(' · ')}`)
  out.push(...other)
  return out.length ? out.join(' · ') : 'cel osiągnięty'
}
