import { createVuetify } from 'vuetify'

export const vuetify = createVuetify({
  theme: {
    defaultTheme: 'system',
    themes: {
      light: { colors: { primary: '#2E7D32', secondary: '#546E7A' } },
      dark: { colors: { primary: '#81C784', secondary: '#90A4AE' } },
    },
  },
  defaults: {
    VBtn: { variant: 'flat' },
    VTextField: { variant: 'outlined', density: 'comfortable' },
    VSelect: { variant: 'outlined', density: 'comfortable' },
  },
})
