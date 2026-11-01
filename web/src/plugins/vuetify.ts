import { createVuetify } from 'vuetify'
import { loadChoice, themeName, VUETIFY_THEMES } from '../theme'

export const vuetify = createVuetify({
  theme: {
    defaultTheme: themeName(loadChoice()),
    themes: VUETIFY_THEMES,
  },
  defaults: {
    VBtn: { variant: 'flat' },
    VTextField: { variant: 'outlined', density: 'comfortable' },
    VSelect: { variant: 'outlined', density: 'comfortable' },
  },
})
