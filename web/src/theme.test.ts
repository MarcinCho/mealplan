import { afterEach, describe, expect, it, vi } from 'vitest'
import { DEFAULT_PALETTE, loadChoice, PALETTES, parseChoice, saveChoice, themeName, VUETIFY_THEMES } from './theme'

afterEach(() => {
  vi.unstubAllGlobals()
  localStorage.clear()
})

function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

describe('themeName', () => {
  it('keeps the built-in names for the classic palette', () => {
    expect(themeName({ palette: 'classic', mode: 'light' })).toBe('light')
    expect(themeName({ palette: 'classic', mode: 'dark' })).toBe('dark')
  })
  it('prefixes other palettes', () => {
    expect(themeName({ palette: 'plum', mode: 'dark' })).toBe('plum-dark')
    expect(themeName({ palette: 'navy', mode: 'light' })).toBe('navy-light')
  })
})

describe('parseChoice', () => {
  it('falls back to the OS preference when nothing is stored', () => {
    expect(parseChoice(null, true)).toEqual({ palette: 'mealplan', mode: 'dark' })
    expect(parseChoice(null, false)).toEqual({ palette: 'mealplan', mode: 'light' })
  })
  it('reads a stored choice', () => {
    expect(parseChoice('{"palette":"navy","mode":"dark"}', false)).toEqual({ palette: 'navy', mode: 'dark' })
  })
  it('ignores garbage, unknown palettes and wrong types', () => {
    expect(parseChoice('not json', false)).toEqual({ palette: 'mealplan', mode: 'light' })
    expect(parseChoice('{"palette":"neon","mode":"dark"}', false)).toEqual({ palette: 'mealplan', mode: 'dark' })
    expect(parseChoice('{"palette":42,"mode":"sepia"}', true)).toEqual({ palette: 'mealplan', mode: 'dark' })
    expect(parseChoice('null', false)).toEqual({ palette: 'mealplan', mode: 'light' })
  })
})

describe('storage', () => {
  it('round-trips a choice', () => {
    saveChoice({ palette: 'navy', mode: 'dark' })
    expect(loadChoice()).toEqual({ palette: 'navy', mode: 'dark' })
  })
  it('survives a storage that throws', () => {
    vi.stubGlobal('localStorage', {
      getItem: () => {
        throw new Error('blocked')
      },
      setItem: () => {
        throw new Error('blocked')
      },
    })
    expect(() => saveChoice({ palette: 'plum', mode: 'light' })).not.toThrow()
    expect(loadChoice().palette).toBe('mealplan')
  })
})

describe('palettes', () => {
  it('defines accent and on-accent in every registered theme', () => {
    for (const [name, def] of Object.entries(VUETIFY_THEMES)) {
      const c = def.colors as Record<string, string>
      expect(c.accent, `${name}.accent`).toMatch(/^#[0-9A-Fa-f]{6}$/)
      expect(c['on-accent'], `${name}.on-accent`).toMatch(/^#[0-9A-Fa-f]{6}$/)
    }
  })

  it('has the Mealplan brand palette first and as default', () => {
    expect(PALETTES[0].id).toBe('mealplan')
    expect(DEFAULT_PALETTE).toBe('mealplan')
    const c = VUETIFY_THEMES['mealplan-light'].colors as Record<string, string>
    expect(c.primary).toBe('#4F6B3F')
    expect(c.accent).toBe('#7B9A68')
    expect(c.background).toBe('#F8F9F7')
  })

  it('registers a light and a dark theme per palette under the names themeName returns', () => {
    for (const p of PALETTES) {
      for (const mode of ['light', 'dark'] as const) {
        const def = VUETIFY_THEMES[themeName({ palette: p.id, mode })]
        expect(def, `${p.id}-${mode}`).toBeDefined()
        expect(def.dark).toBe(mode === 'dark')
      }
    }
  })

  it('defines the colors the shell uses and keeps text readable (WCAG AA 4.5:1)', () => {
    for (const [name, def] of Object.entries(VUETIFY_THEMES)) {
      const c = def.colors as Record<string, string>
      for (const key of ['primary', 'secondary', 'bar', 'background', 'surface', 'error']) {
        expect(c[key], `${name}.${key}`).toMatch(/^#[0-9A-Fa-f]{6}$/)
      }
      expect(contrast(c.primary, c['on-primary']), `${name} primary`).toBeGreaterThanOrEqual(4.5)
      expect(contrast(c.secondary, c['on-secondary']), `${name} secondary`).toBeGreaterThanOrEqual(4.5)
      expect(contrast(c.bar, c['on-bar']), `${name} bar`).toBeGreaterThanOrEqual(4.5)
      if (c['on-accent']) expect(contrast(c.accent, c['on-accent']), `${name} accent (decorative, WCAG 3:1)`).toBeGreaterThanOrEqual(3)
    }
  })

  it('offers the Mealplan brand palette first and four others, each with swatches for the picker', () => {
    expect(PALETTES.map((p) => p.id)).toEqual(['mealplan', 'classic', 'plum', 'earth', 'navy'])
    for (const p of PALETTES) expect(p.swatch).toHaveLength(4)
  })
})
