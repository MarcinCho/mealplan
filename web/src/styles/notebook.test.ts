import { describe, expect, it } from 'vitest'
import css from './notebook.css?raw'
// vite.config.ts lets vitest keep notebook.css content (it blanks CSS by default).
import main from '../main.ts?raw'
import html from '../../index.html?raw'

const FONT_IMPORTS = [
  ...[400, 500, 700].flatMap((w) => [`@fontsource/red-hat-text/latin-${w}.css`, `@fontsource/red-hat-text/latin-ext-${w}.css`]),
  ...[600, 700].flatMap((w) => [`@fontsource/red-hat-display/latin-${w}.css`, `@fontsource/red-hat-display/latin-ext-${w}.css`]),
]

describe('notebook.css', () => {
  it('sets body and heading fonts from the theme', () => {
    expect(css).toMatch(/html[^{]*\{[^}]*font-family:\s*var\(--v-font-text\)/)
    for (let n = 1; n <= 6; n++) expect(css).toContain(`.text-h${n}`)
    expect(css).toContain('var(--v-font-display)')
  })

  it('defines the notebook shapes', () => {
    expect(css).toContain('--mp-radius-control: 6px')
    expect(css).toContain('--mp-radius-card: 10px')
    // The :root prefix keeps the rules above Vuetify's component CSS, which the built bundle loads later.
    for (const sel of [':root .v-btn:not', ':root .v-card', ':root .v-field', ':root .v-chip', ':root .v-alert', ':root .v-btn-group', ':root .v-menu > .v-overlay__content', ':root .v-snackbar__wrapper', ':root .v-dialog > .v-overlay__content', ':root .v-dialog > .v-overlay__content > .v-card', ':root .v-list']) {
      expect(css).toContain(sel)
    }
    const shapeBlock = css.slice(css.indexOf('/* ---------- shapes'), css.indexOf('/* A plain flat button'))
    const selectors = shapeBlock.replace(/\/\*[\s\S]*?\*\//g, '').split('{').slice(0, -1).flatMap((chunk) => chunk.split('}').pop()!.split(','))
    for (const s of selectors) expect(s.trim()).toMatch(/^:root /)
  })

  it('leaves buttons inside a group square so only the group rounds its outer corners', () => {
    expect(css).toContain(':root .v-btn:not(.v-btn--icon):not(.v-btn-group .v-btn)')
  })

  it('bundles the fonts and never loads them from another server', () => {
    expect(FONT_IMPORTS).toHaveLength(10)
    for (const f of FONT_IMPORTS) expect(main).toContain(f)
    expect(css + main + html).not.toMatch(/fonts\.googleapis|fonts\.gstatic|@import\s+url\(\s*['"]?https?:/)
  })
})
