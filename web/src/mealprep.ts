import type { PlanEntry, PrepAllocation, PrepSession } from './api/types'
import { t } from './messages'
import { SLOTS } from './weeks'

const pad = (n: number) => String(n).padStart(2, '0')

function parse(date: string): Date {
  const [y, m, d] = date.split('-').map(Number)
  return new Date(y, m - 1, d)
}

function fmt(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

// Monday of the shopping week a session on `date` belongs to: Mon-Fri their own week, a weekend the coming one.
export function defaultWeek(date: string): string {
  const d = parse(date)
  const wd = d.getDay() // 0 = Sunday
  const shift = wd === 6 ? 2 : wd === 0 ? 1 : -(wd - 1)
  return fmt(new Date(d.getFullYear(), d.getMonth(), d.getDate() + shift))
}

const ROMAN = ['I', 'II', 'III', 'IV', 'V', 'VI', 'VII', 'VIII', 'IX', 'X', 'XI', 'XII']

export function sessionTitle(date: string): string {
  const d = parse(date)
  return `Gotowanie ${d.getDate()} ${ROMAN[d.getMonth()]}`
}

const STALE_DAYS = 4

// Sessions cooked more than 4 days before today that still have portions nobody has claimed.
export function staleSessions(sessions: PrepSession[], today: string): PrepSession[] {
  const t = parse(today).getTime()
  return sessions.filter((s) => s.remaining > 0 && Math.round((t - parse(s.date).getTime()) / 86400000) > STALE_DAYS)
}

const DAYS = ['Nd', 'Pn', 'Wt', 'Śr', 'Cz', 'Pt', 'So']

export function allocationLabel(a: Pick<PrepAllocation, 'date' | 'slot' | 'servings'>): string {
  const slot = (SLOTS.find((s) => s.value === a.slot)?.title ?? a.slot).toLocaleLowerCase('pl-PL')
  return `${DAYS[parse(a.date).getDay()]} ${slot} ×${a.servings}`
}

// Small badge for entries that do not come from a recipe or free text.
export function entryChip(e: Pick<PlanEntry, 'kind' | 'prepDishId' | 'freezerItemId'>): string {
  if (e.kind === 'out') return t.chipOut
  if (e.kind === 'prep') return e.freezerItemId ? t.chipFreezer : t.chipPrep
  return ''
}

// What the plan shows as the entry's name: an eaten-out entry without a description is just "Na mieście".
export function entryTitle(e: Pick<PlanEntry, 'kind' | 'freeText'>, recipeTitle: string | null): string {
  if (e.kind === 'out' && !e.freeText.trim()) return t.entryOut
  return recipeTitle ?? e.freeText
}
