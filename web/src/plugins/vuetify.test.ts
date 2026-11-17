import { describe, expect, it } from 'vitest'
import { vuetify } from './vuetify'

describe('vuetify defaults', () => {
  it('drops the Material 3 pills and borders', () => {
    const defaults = vuetify.defaults.value as Record<string, Record<string, unknown>>
    expect(JSON.stringify(defaults)).not.toContain('"rounded"')
    expect(defaults.VCard.border).toBeUndefined()
    expect(defaults.VBtn.variant).toBe('flat')
    expect(defaults.VTextField.variant).toBe('outlined')
  })
})
