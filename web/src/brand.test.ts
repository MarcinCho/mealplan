import { describe, expect, it } from 'vitest'
import { logoVariantFor } from './brand'
import { PALETTES } from './theme'

describe('logoVariantFor', () => {
  it('uses the light-text wordmark on dark backgrounds', () => {
    expect(logoVariantFor('#233831')).toBe('dark')
    expect(logoVariantFor('#1E2622')).toBe('dark')
  })
  it('uses the dark-text wordmark on light backgrounds', () => {
    expect(logoVariantFor('#FFFFFF')).toBe('light')
    expect(logoVariantFor('#F8F9F7')).toBe('light')
  })
  it('follows the classic palette white bar', () => {
    const classic = PALETTES.find((p) => p.id === 'classic')!
    expect(logoVariantFor((classic.light.colors as Record<string, string>).bar)).toBe('light')
  })
  it('accepts lowercase hex', () => {
    expect(logoVariantFor('#233831'.toLowerCase())).toBe('dark')
  })
  it('picks the right variant for every palette background', () => {
    for (const p of PALETTES) {
      expect(logoVariantFor((p.light.colors as Record<string, string>).background), `${p.id} light`).toBe('light')
      expect(logoVariantFor((p.dark.colors as Record<string, string>).background), `${p.id} dark`).toBe('dark')
    }
  })
})
