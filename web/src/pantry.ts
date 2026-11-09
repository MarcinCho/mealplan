import type { PantryItem, ShoppingItem } from './api/types'
import { formatQuantity } from './stores/recipes'
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

// Empty clears the date; otherwise YYYY-MM-DD with a sane year (typing a year emits junk dates like 0002-...).
export function validExpiry(v: string | null | undefined): boolean {
  if (!v) return true
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(v)
  if (!m || Number(m[1]) < 1900) return false
  const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]))
  return d.getMonth() === Number(m[2]) - 1 && d.getDate() === Number(m[3])
}

export interface SerialSaver<T> {
  set(v: T): void
  cancel(): void
  readonly pending: boolean
}

// Collects rapid changes into one send after `delay` ms. At most one send is in flight; changes made
// meanwhile are merged and sent afterwards with the latest value. A failure drops queued changes and
// reports once, so the caller can roll back to what the server has.
export function createSerialSaver<T>(opts: {
  send: (v: T) => Promise<unknown>
  delay: number
  onError: (e: unknown) => void
  onIdle?: () => void
  merge?: (prev: T, next: T) => T
}): SerialSaver<T> {
  let queued: { v: T } | null = null
  let timer: ReturnType<typeof setTimeout> | undefined
  let inflight = false
  let gen = 0 // bumped by cancel(): a send already in flight no longer reports

  async function flush() {
    timer = undefined
    if (inflight || !queued) return
    const { v } = queued
    queued = null
    inflight = true
    const mine = gen
    try {
      await opts.send(v)
    } catch (e) {
      if (mine !== gen) return
      queued = null
      clearTimeout(timer)
      timer = undefined
      inflight = false
      opts.onError(e)
      return
    }
    if (mine !== gen) return
    inflight = false
    if (queued && timer === undefined) void flush()
    else if (!queued && timer === undefined) opts.onIdle?.()
  }

  return {
    set(v: T) {
      queued = { v: queued && opts.merge ? opts.merge(queued.v, v) : v }
      clearTimeout(timer)
      timer = setTimeout(flush, opts.delay)
    },
    cancel() {
      gen++
      queued = null
      clearTimeout(timer)
      timer = undefined
      inflight = false
    },
    get pending() {
      return queued !== null || inflight || timer !== undefined
    },
  }
}

// One shopping row's amount text: covered by the pantry, partly covered, or as generated.
export function shoppingLine(item: Pick<ShoppingItem, 'quantity' | 'unit' | 'haveQuantity'>): string {
  const have = item.haveQuantity
  if (have !== null && have !== undefined && have > 0) {
    if (item.quantity === 0) return `${t.haveAtHome}: ${formatQuantity(have, item.unit)}`
    if (item.quantity !== null) return `${t.buyPartial} ${formatQuantity(item.quantity, item.unit)} (${t.haveShort} ${formatQuantity(have, item.unit)})`
  }
  return formatQuantity(item.quantity, item.unit)
}
