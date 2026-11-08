import { createVuetify } from 'vuetify'
import { loadChoice, themeName, VUETIFY_THEMES } from '../theme'

export const vuetify = createVuetify({
  theme: {
    defaultTheme: themeName(loadChoice()),
    themes: VUETIFY_THEMES,
  },
  // Material 3 feel: large corners, flat tonal surfaces with a hairline border instead of shadows.
  defaults: {
    VBtn: { variant: 'flat', rounded: 'pill' },
    VCard: { rounded: 'xl', variant: 'flat', border: true },
    VAlert: { rounded: 'lg' },
    VChip: { rounded: 'lg' },
    VList: { rounded: 'lg' },
    VTextField: { variant: 'outlined', density: 'comfortable', rounded: 'lg' },
    VSelect: { variant: 'outlined', density: 'comfortable', rounded: 'lg' },
  },
})
