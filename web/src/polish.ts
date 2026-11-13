// Counts, numbers and dates as the interface shows them, in the chosen language
// ("2 porcje" / "2 portions", "12 lis" / "12 Nov").
import { intl } from './i18n'
import { t } from './messages'

export { form } from './plural'

export const fmtNumber = (n: number): string => n.toLocaleString(intl, { maximumFractionDigits: 3 })

export function portionWord(n: number): string {
  return t.portionWord(n)
}

export function portions(n: number): string {
  return `${fmtNumber(n)} ${portionWord(n)}`
}

// Kitchen units are stored in their Polish base form; metric units never change.
export function unitWord(n: number, unit: string): string {
  return t.unitWord(n, unit)
}

// "12 lis" for a date in the current year, "12 lis 2027" otherwise; expects a yyyy-mm-dd calendar date.
export function shortDate(iso: string, now: Date = new Date()): string {
  const [y, m, d] = iso.split('-').map(Number)
  if (!y || !m || !d) return iso
  const date = new Date(y, m - 1, d)
  const opts: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short' }
  if (y !== now.getFullYear()) opts.year = 'numeric'
  return date.toLocaleDateString(intl, opts).replace('.', '')
}
