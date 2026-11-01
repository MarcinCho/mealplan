import type { ThemeDefinition } from 'vuetify'

export type PaletteId = 'classic' | 'plum' | 'earth' | 'navy'
export type Mode = 'light' | 'dark'
export interface Choice {
  palette: PaletteId
  mode: Mode
}

export interface Palette {
  id: PaletteId
  swatch: string[]
  light: ThemeDefinition
  dark: ThemeDefinition
}

const STORAGE_KEY = 'mealplan.theme'

const statusLight = { error: '#B3261E', info: '#1565C0', success: '#2E7D32', warning: '#9A5B00' }
const statusDark = { error: '#F2B8B5', info: '#90CAF9', success: '#A5D6A7', warning: '#FFCC80' }

function define(dark: boolean, colors: Record<string, string>): ThemeDefinition {
  return { dark, colors: { ...(dark ? statusDark : statusLight), ...colors } }
}

// `bar` is the app bar color (own theme color so each palette can brand it).
export const PALETTES: Palette[] = [
  {
    id: 'classic',
    swatch: ['#2E7D32', '#546E7A', '#81C784', '#FFFFFF'],
    light: define(false, {
      background: '#FFFFFF', surface: '#FFFFFF',
      primary: '#2E7D32', 'on-primary': '#FFFFFF',
      secondary: '#546E7A', 'on-secondary': '#FFFFFF',
      bar: '#FFFFFF', 'on-bar': '#1F1F1F',
    }),
    dark: define(true, {
      background: '#121212', surface: '#212121',
      primary: '#81C784', 'on-primary': '#0B2A0D',
      secondary: '#90A4AE', 'on-secondary': '#10181C',
      bar: '#212121', 'on-bar': '#EDEDED',
    }),
  },
  {
    id: 'plum',
    swatch: ['#290044', '#500071', '#8A2A78', '#EEE3B0'],
    light: define(false, {
      background: '#FBF7E8', surface: '#FFFFFF', 'surface-variant': '#EEE3B0',
      primary: '#500071', 'on-primary': '#FFFFFF',
      secondary: '#8A2A78', 'on-secondary': '#FFFFFF',
      bar: '#290044', 'on-bar': '#EEE3B0',
    }),
    dark: define(true, {
      background: '#1A0030', surface: '#290044', 'surface-variant': '#3A0A5A',
      primary: '#EEE3B0', 'on-primary': '#290044',
      secondary: '#D08BC3', 'on-secondary': '#2B0A26',
      bar: '#22003A', 'on-bar': '#EEE3B0',
    }),
  },
  {
    id: 'earth',
    swatch: ['#09251C', '#A55704', '#F3D2AF', '#F8EADC'],
    light: define(false, {
      background: '#F8EADC', surface: '#FFFBF6', 'surface-variant': '#F3D2AF',
      primary: '#A55704', 'on-primary': '#FFFFFF',
      secondary: '#09251C', 'on-secondary': '#F8EADC',
      bar: '#09251C', 'on-bar': '#F3D2AF',
    }),
    dark: define(true, {
      background: '#06180F', surface: '#0F3326', 'surface-variant': '#1A4636',
      primary: '#F3D2AF', 'on-primary': '#09251C',
      secondary: '#E0913F', 'on-secondary': '#2A1600',
      bar: '#09251C', 'on-bar': '#F3D2AF',
    }),
  },
  {
    id: 'navy',
    swatch: ['#27374D', '#526D82', '#9DB2BF', '#DDE6ED'],
    light: define(false, {
      background: '#EEF2F6', surface: '#FFFFFF', 'surface-variant': '#DDE6ED',
      primary: '#27374D', 'on-primary': '#FFFFFF',
      secondary: '#526D82', 'on-secondary': '#FFFFFF',
      bar: '#27374D', 'on-bar': '#DDE6ED',
    }),
    dark: define(true, {
      background: '#17202C', surface: '#27374D', 'surface-variant': '#33475F',
      primary: '#9DB2BF', 'on-primary': '#14202E',
      secondary: '#7FA0B8', 'on-secondary': '#0E1A26',
      bar: '#1F2D41', 'on-bar': '#DDE6ED',
    }),
  },
]

export function themeName(c: Choice): string {
  return c.palette === 'classic' ? c.mode : `${c.palette}-${c.mode}`
}

// Every palette registers a light and a dark theme with Vuetify; classic overrides the built-ins.
export const VUETIFY_THEMES: Record<string, ThemeDefinition> = Object.fromEntries(
  PALETTES.flatMap((p) => [
    [themeName({ palette: p.id, mode: 'light' }), p.light],
    [themeName({ palette: p.id, mode: 'dark' }), p.dark],
  ]),
)

export function parseChoice(raw: string | null, prefersDark: boolean): Choice {
  const fallback: Choice = { palette: 'classic', mode: prefersDark ? 'dark' : 'light' }
  if (!raw) return fallback
  try {
    const v = JSON.parse(raw)
    if (!v || typeof v !== 'object') return fallback
    const palette = PALETTES.some((p) => p.id === v.palette) ? (v.palette as PaletteId) : fallback.palette
    const mode: Mode = v.mode === 'light' || v.mode === 'dark' ? v.mode : fallback.mode
    return { palette, mode }
  } catch {
    return fallback
  }
}

function osPrefersDark(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: dark)').matches
}

// Storage can be blocked (private windows, cleared site data); the theme must still work.
export function loadChoice(): Choice {
  let raw: string | null = null
  try {
    raw = localStorage.getItem(STORAGE_KEY)
  } catch {
    raw = null
  }
  return parseChoice(raw, osPrefersDark())
}

export function saveChoice(c: Choice): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(c))
  } catch {
    // per-viewer convenience only
  }
}
