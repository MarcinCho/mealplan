import type { Slot } from './api/types'

const pad = (n: number) => String(n).padStart(2, '0')

export function isoDate(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function parse(date: string): Date {
  const [y, m, d] = date.split('-').map(Number)
  return new Date(y, m - 1, d)
}

// mondayOf returns the Monday of the week containing d (weeks run Monday to Sunday).
export function mondayOf(d: Date): string {
  const day = (d.getDay() + 6) % 7
  return isoDate(new Date(d.getFullYear(), d.getMonth(), d.getDate() - day))
}

export function shiftWeek(week: string, n: number): string {
  const d = parse(week)
  return isoDate(new Date(d.getFullYear(), d.getMonth(), d.getDate() + 7 * n))
}

export function weekDates(week: string): string[] {
  const d = parse(week)
  return Array.from({ length: 7 }, (_, i) => isoDate(new Date(d.getFullYear(), d.getMonth(), d.getDate() + i)))
}

export function formatDay(date: string): string {
  return parse(date).toLocaleDateString('pl-PL', { weekday: 'short', day: 'numeric', month: 'short' })
}

export const SLOTS: { value: Slot; title: string }[] = [
  { value: 'breakfast', title: 'Śniadanie' },
  { value: 'lunch', title: 'Obiad' },
  { value: 'dinner', title: 'Kolacja' },
  { value: 'snack', title: 'Przekąska' },
]
