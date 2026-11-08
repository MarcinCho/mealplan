import type { PantryItem } from './api/types'
import { t } from './messages'
import { isoDate } from './weeks'

const REVIEW_DAYS = 30

// True when the pantry has items and was never reviewed, or the last review is over 30 days old.
export function reviewDue(reviewedAt: string | null, itemCount: number, now: Date): boolean {
  if (itemCount <= 0) return false
  if (!reviewedAt) return true
  const at = Date.parse(reviewedAt)
  if (Number.isNaN(at)) return true
  return now.getTime() - at > REVIEW_DAYS * 86400000
}

export function stepFor(unit: string): number {
  return unit === 'g' || unit === 'ml' ? 50 : 1
}

const num = (n: number) => n.toLocaleString('pl-PL', { maximumFractionDigits: 2 })

export function formatAmount(quantity: number, unit: string): string {
  if (quantity >= 1000 && unit === 'g') return `${num(quantity / 1000)} kg`
  if (quantity >= 1000 && unit === 'ml') return `${num(quantity / 1000)} l`
  return unit ? `${num(quantity)} ${unit}` : num(quantity)
}

export function formatStock(item: Pick<PantryItem, 'quantity' | 'unit' | 'staple'>): string {
  if (item.quantity === null || item.quantity === undefined) return item.staple ? t.pantryStaple : ''
  return formatAmount(item.quantity, item.unit)
}

// expiresOn is a local calendar date; expired means strictly before today.
export function isExpired(expiresOn: string, now: Date): boolean {
  return expiresOn !== '' && expiresOn < isoDate(now)
}

export const PANTRY_UNITS = [
  'g', 'kg', 'ml', 'l', 'szt', 'łyżka', 'łyżeczka', 'szklanka', 'opakowanie', 'puszka', 'kostka',
  'ząbek', 'plaster', 'szczypta', 'pęczek', 'garść', 'główka', 'kromka',
]
