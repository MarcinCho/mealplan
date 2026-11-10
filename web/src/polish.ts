// Polish number agreement for counts shown in the UI ("1 porcja", "2 porcje", "5 porcji", "1,5 porcji").

// [one, few (2-4), many (5+, 12-14), fraction (1,5)]
type Forms = readonly [string, string, string, string]

export function form(n: number, f: Forms): string {
  if (!Number.isInteger(n)) return f[3]
  if (n === 1) return f[0]
  const last = Math.abs(n) % 10
  const tens = Math.abs(n) % 100
  return last >= 2 && last <= 4 && !(tens >= 12 && tens <= 14) ? f[1] : f[2]
}

export const fmtNumber = (n: number): string => n.toLocaleString('pl-PL', { maximumFractionDigits: 3 })

const PORTION: Forms = ['porcja', 'porcje', 'porcji', 'porcji']

export function portionWord(n: number): string {
  return form(n, PORTION)
}

export function portions(n: number): string {
  return `${fmtNumber(n)} ${portionWord(n)}`
}

// Kitchen units stored in their base form; metric units and "szt" never change.
const UNITS: Record<string, Forms> = {
  łyżka: ['łyżka', 'łyżki', 'łyżek', 'łyżki'],
  łyżeczka: ['łyżeczka', 'łyżeczki', 'łyżeczek', 'łyżeczki'],
  szklanka: ['szklanka', 'szklanki', 'szklanek', 'szklanki'],
  ząbek: ['ząbek', 'ząbki', 'ząbków', 'ząbka'],
  plaster: ['plaster', 'plastry', 'plastrów', 'plastra'],
  szczypta: ['szczypta', 'szczypty', 'szczypt', 'szczypty'],
  opakowanie: ['opakowanie', 'opakowania', 'opakowań', 'opakowania'],
  puszka: ['puszka', 'puszki', 'puszek', 'puszki'],
  pęczek: ['pęczek', 'pęczki', 'pęczków', 'pęczka'],
  garść: ['garść', 'garście', 'garści', 'garści'],
  kostka: ['kostka', 'kostki', 'kostek', 'kostki'],
  główka: ['główka', 'główki', 'główek', 'główki'],
  kromka: ['kromka', 'kromki', 'kromek', 'kromki'],
}

export function unitWord(n: number, unit: string): string {
  const f = UNITS[unit]
  return f ? form(n, f) : unit
}

// "12 lis" for a date in the current year, "12 lis 2027" otherwise; expects a yyyy-mm-dd calendar date.
export function shortDate(iso: string, now: Date = new Date()): string {
  const [y, m, d] = iso.split('-').map(Number)
  if (!y || !m || !d) return iso
  const date = new Date(y, m - 1, d)
  const opts: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short' }
  if (y !== now.getFullYear()) opts.year = 'numeric'
  return date.toLocaleDateString('pl-PL', opts).replace('.', '')
}
