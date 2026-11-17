// Servings scaling: parse the ?servings= query and round scaled amounts the way a cook would.

export const MAX_SERVINGS = 100

const SMALL: Record<string, string> = { g: 'kg', ml: 'l' }
const BIG: Record<string, string> = { kg: 'g', l: 'ml' }
const SPOONS = new Set(['łyżka', 'łyżeczka', 'szklanka'])
const PIECES = new Set([
  'szt', '', 'ząbek', 'plaster', 'opakowanie', 'puszka', 'pęczek', 'garść', 'kostka', 'główka', 'kromka',
])

type Amount = { quantity: number | null; unit: string }

// Returns a whole number 1..MAX_SERVINGS from a single all-digit string, otherwise `base`.
export function parseServings(raw: unknown, base: number): number {
  if (typeof raw !== 'string' || !/^\d+$/.test(raw)) return base
  const n = Number(raw)
  return n >= 1 && n <= MAX_SERVINGS ? n : base
}

function step(v: number, s: number): number {
  return Math.round(Math.round(v / s) * s * 1000) / 1000
}

function roundAmount(v: number, unit: string): { quantity: number; unit: string } {
  if (unit in SMALL) {
    if (v >= 1000) return roundAmount(v / 1000, SMALL[unit])
    const r = v < 10 ? Math.max(step(v, 0.5), 0.5) : v < 100 ? step(v, 1) : step(v, 10)
    return r >= 1000 ? { quantity: r / 1000, unit: SMALL[unit] } : { quantity: r, unit }
  }
  if (unit in BIG) {
    if (v < 1) return roundAmount(v * 1000, BIG[unit])
    return { quantity: step(v, 0.05), unit }
  }
  if (SPOONS.has(unit)) return { quantity: Math.max(v < 1 ? step(v, 0.25) : step(v, 0.5), 0.25), unit }
  if (unit === 'szczypta') return { quantity: Math.max(step(v, 1), 1), unit }
  if (PIECES.has(unit)) return { quantity: Math.max(step(v, 0.5), 0.5), unit }
  return { quantity: Math.max(v < 10 ? step(v, 0.1) : step(v, 1), 0.1), unit }
}

export function scaleAmount(quantity: number | null, unit: string, factor: number): Amount {
  if (factor === 1 || quantity === null) return { quantity, unit }
  const v = quantity * factor
  if (v === 0) return { quantity: 0, unit }
  return roundAmount(v, unit)
}
