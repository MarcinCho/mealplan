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

// Share of the goal reached, for a progress bar: 0..100, null when there is no goal.
export function goalPercent(total: number | null, goal: number | null): number | null {
  if (goal === null || goal === undefined || goal <= 0) return null
  if (total === null || total === undefined) return 0
  return Math.min(100, Math.max(0, (total / goal) * 100))
}

function plural(n: number, one: string, few: string, many: string): string {
  if (n === 1) return one
  const last = n % 10
  const tens = n % 100
  return last >= 2 && last <= 4 && !(tens >= 12 && tens <= 14) ? few : many
}

// Warnings that tell the user the day's total is not the whole truth.
export function dayWarnings(d: DaySummary): string[] {
  const out: string[] = []
  if (d.missingMeals > 0) out.push(`${d.missingMeals} ${plural(d.missingMeals, 'posiłek', 'posiłki', 'posiłków')} bez wartości`)
  if (d.missingLog > 0) out.push(`${d.missingLog} ${plural(d.missingLog, 'pozycja', 'pozycje', 'pozycji')} bez wartości`)
  if (d.approximate) out.push('wartości przybliżone')
  return out
}

// Mirrors the API: quantity > 0 and <= 10000.
export function validQuantity(q: number | null | undefined): boolean {
  return typeof q === 'number' && Number.isFinite(q) && q > 0 && q <= 10000
}

// One-line day digest for the week plan: "≥1850 / 2000 kcal · białko 92 g".
// A total built from incomplete data is marked as a lower bound, never shown as exact.
export function dayLine(d: DaySummary): string {
  const val = (k: keyof Macros, unit: 'kcal' | 'g') => {
    const v = d.total[k]
    return v === null || v === undefined ? '—' : `${d.incomplete[k] ? '≥' : ''}${formatNumber(v, unit)}`
  }
  const goal = d.goal.kcal !== null && d.goal.kcal !== undefined ? ` / ${formatNumber(d.goal.kcal, 'kcal')}` : ''
  return `${val('kcal', 'kcal')}${goal} kcal · białko ${val('protein', 'g')} g`
}
