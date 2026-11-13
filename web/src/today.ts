import type { PlanEntry, Slot } from './api/types'
import { t } from './messages'

const SLOT_ORDER: Slot[] = ['breakfast', 'lunch', 'dinner', 'snack']

// "Dziś na obiad" / "Today for lunch": the slot name as it follows todayOn/dayOn.
export const SLOT_ACCUSATIVE: Record<Slot, string> = t.slotOn

export const SLOT_ICON: Record<Slot, string> = {
  breakfast: 'mdi-coffee-outline',
  lunch: 'mdi-pot-steam-outline',
  dinner: 'mdi-silverware-fork-knife',
  snack: 'mdi-cookie-outline',
}

export function entriesOfDay(entries: PlanEntry[], date: string): PlanEntry[] {
  return entries
    .filter((e) => e.date === date)
    .sort((a, b) => SLOT_ORDER.indexOf(a.slot) - SLOT_ORDER.indexOf(b.slot))
}

export type Hero = { kind: 'empty' } | { kind: 'finished' } | { kind: 'next'; entry: PlanEntry }

// The hero shows the first meal of the day that is not cooked yet.
export function heroOf(entries: PlanEntry[], date: string): Hero {
  const day = entriesOfDay(entries, date)
  if (day.length === 0) return { kind: 'empty' }
  const next = day.find((e) => !e.done)
  return next ? { kind: 'next', entry: next } : { kind: 'finished' }
}

// Main meals without an entry get a dashed "add" placeholder; snacks are optional.
export function freeSlots(entries: PlanEntry[], date: string): Slot[] {
  const taken = new Set(entriesOfDay(entries, date).map((e) => e.slot))
  return (['breakfast', 'lunch', 'dinner'] as Slot[]).filter((s) => !taken.has(s))
}

const AVATAR_COLORS = ['#A55704', '#2F6B55', '#7A4E9C', '#3B6EA8', '#B0473A', '#5C7A29']

export function avatarColor(memberId: string, memberIds: string[]): string {
  const i = memberIds.indexOf(memberId)
  return AVATAR_COLORS[(i < 0 ? 0 : i) % AVATAR_COLORS.length]
}

export function initial(name: string): string {
  return [...name.trim()][0]?.toUpperCase() ?? '?'
}

export function greeting(hour: number): string {
  if (hour < 5) return t.greetingNight
  if (hour < 18) return t.greetingDay
  return t.greetingEvening
}

// Case-insensitive title match; titles that start with the query come first.
export function suggest<T extends { title: string }>(items: T[], query: string, limit = 5): T[] {
  const q = query.trim().toLocaleLowerCase('pl-PL')
  if (!q) return []
  const hits = items.filter((i) => i.title.toLocaleLowerCase('pl-PL').includes(q))
  const starts = (i: T) => (i.title.toLocaleLowerCase('pl-PL').startsWith(q) ? 0 : 1)
  return hits.sort((a, b) => starts(a) - starts(b)).slice(0, limit)
}
