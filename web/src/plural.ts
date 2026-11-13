// Polish number agreement: [one, few (2-4), many (5+, 12-14), fraction (1,5)].
export type Forms = readonly [string, string, string, string]

export function form(n: number, f: Forms): string {
  if (!Number.isInteger(n)) return f[3]
  if (n === 1) return f[0]
  const last = Math.abs(n) % 10
  const tens = Math.abs(n) % 100
  return last >= 2 && last <= 4 && !(tens >= 12 && tens <= 14) ? f[1] : f[2]
}

// English: one, otherwise many (fractions too: "1.5 portions").
export function en(n: number, one: string, many: string): string {
  return n === 1 ? one : many
}
