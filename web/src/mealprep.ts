import type { FreezerItem, PlanEntry, PrepAllocation, PrepSession } from './api/types'
import { intl } from './i18n'
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


export function allocationLabel(a: Pick<PrepAllocation, 'date' | 'slot' | 'servings'>): string {
  const slot = (SLOTS.find((s) => s.value === a.slot)?.title ?? a.slot).toLocaleLowerCase(intl)
  return `${t.weekdaysShortSun[parse(a.date).getDay()]} ${slot} ×${a.servings}`
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

export interface PrepSource { value: string; title: string; remaining: number }

// Sources a prep entry can draw from, as 'd:<dishId>' / 'f:<freezerItemId>' options. While editing, the
// entry's own portions count as available again. A source that is no longer listed (a freezer item that is
// fully planned, a dish with nothing left) still gets an option for the entry being edited.
export function prepSources(
  sessions: PrepSession[],
  freezer: FreezerItem[],
  entry: Pick<PlanEntry, 'kind' | 'servings' | 'freeText' | 'prepDishId' | 'freezerItemId'> | null,
  recipeTitle: string | null = null,
): PrepSource[] {
  const editing = entry?.kind === 'prep' ? entry : null
  const ownValue = editing?.prepDishId ? `d:${editing.prepDishId}` : editing?.freezerItemId ? `f:${editing.freezerItemId}` : null
  const own = editing?.servings ?? 0
  const list: PrepSource[] = []
  for (const s of sessions)
    for (const d of s.dishes) {
      const value = `d:${d.id}`
      const remaining = d.remaining + (value === ownValue ? own : 0)
      if (remaining > 0) list.push({ value, title: `${d.recipeTitle} · ${t.entryPrepLeft} ${remaining}`, remaining })
    }
  for (const f of freezer) {
    const value = `f:${f.id}`
    const remaining = f.remaining + (value === ownValue ? own : 0)
    if (remaining > 0) list.push({ value, title: `❄ ${f.title} · ${t.entryPrepLeft} ${remaining}`, remaining })
  }
  if (editing && ownValue && !list.some((x) => x.value === ownValue)) {
    const name = recipeTitle ?? editing.freeText
    const title = ownValue.startsWith('f:') ? `❄ ${name}` : name
    list.push({ value: ownValue, title: `${title} · ${t.entryPrepLeft} ${own}`, remaining: own })
  }
  return list
}
