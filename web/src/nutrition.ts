import type { DaySummary, Macros } from './api/types'
import { intl } from './i18n'
import { t } from './messages'

const KEYS: (keyof Macros)[] = ['kcal', 'protein', 'carbs', 'fat']


export function formatNumber(v: number | null, unit: 'kcal' | 'g'): string {
  if (v === null || v === undefined) return '—'
  // Grams carry one decimal only below 10 g, where it still means something ("4,5 g", but "139 g").
  const digits = unit === 'kcal' || Math.abs(v) >= 10 ? 0 : 1
  return v.toLocaleString(intl, { maximumFractionDigits: digits })
}

function amount(k: keyof Macros, v: number): string {
  return k === 'kcal' ? `${formatNumber(v, 'kcal')} kcal` : `${formatNumber(v, 'g')} g ${t.gramsOf[k]}`
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
      other.push(t.noDataAbout(t.aboutOf[k]))
    } else if (d.incomplete[k]) {
      if (total < goal) atLeast.push(amount(k, goal - total))
      else other.push(t.incompleteAbout(t.aboutOf[k]))
    } else if (total < goal) {
      precise.push(amount(k, goal - total))
    }
  }
  if (!anyGoal) return t.noGoal
  const out: string[] = []
  if (precise.length) out.push(t.missing(precise.join(' · ')))
  if (atLeast.length) out.push(t.missingAtLeast(atLeast.join(' · ')))
  out.push(...other)
  return out.length ? out.join(' · ') : t.goalReached
}

// Share of the goal reached, for a progress bar: 0..100, null when there is no goal.
export function goalPercent(total: number | null, goal: number | null): number | null {
  if (goal === null || goal === undefined || goal <= 0) return null
  if (total === null || total === undefined) return 0
  return Math.min(100, Math.max(0, (total / goal) * 100))
}

export function plural(n: number, one: string, few: string, many: string): string {
  if (n === 1) return one
  const last = n % 10
  const tens = n % 100
  return last >= 2 && last <= 4 && !(tens >= 12 && tens <= 14) ? few : many
}

// Warnings that tell the user the day's total is not the whole truth.
export function dayWarnings(d: DaySummary): string[] {
  const out: string[] = []
  if (d.missingMeals > 0) out.push(t.mealsWithoutValues(d.missingMeals))
  if (d.missingLog > 0) out.push(t.logWithoutValues(d.missingLog))
  if (d.approximate) out.push(t.valuesApprox)
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
  return `${val('kcal', 'kcal')}${goal} kcal · ${t.proteinWord} ${val('protein', 'g')} g`
}
