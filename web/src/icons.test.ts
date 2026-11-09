import { describe, expect, it } from 'vitest'
import { ICONS } from './icons'

const files = import.meta.glob(['./**/*.vue', './**/*.ts', '!./**/*.test.ts', '!./icons.ts'], { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('icons', () => {
  it('has an SVG path for every mdi-* name used in the app', () => {
    const used = new Set<string>()
    for (const text of Object.values(files)) for (const m of text.matchAll(/(?<![/\w])mdi-[a-z0-9-]+/g)) used.add(m[0])
    expect(used.size).toBeGreaterThan(20)
    expect([...used].filter((n) => !ICONS[n])).toEqual([])
  })

  it('maps names to SVG paths', () => {
    for (const path of Object.values(ICONS)) expect(path).toMatch(/^M[\d.,\sA-Za-z-]+$/)
  })
})
