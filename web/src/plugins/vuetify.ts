import { h } from 'vue'
import { createVuetify, type IconProps, type IconSet } from 'vuetify'
import { VSvgIcon } from 'vuetify/components'
import { aliases } from 'vuetify/iconsets/mdi-svg'
import { ICONS } from '../icons'
import { loadChoice, themeName, VUETIFY_THEMES } from '../theme'

// Icons are written as mdi-* names in templates and drawn as inline SVG paths from icons.ts,
// so the app ships only the icons it uses instead of the whole icon font.
const mdiNamed: IconSet = {
  component: (props: IconProps) => h(VSvgIcon, { ...props, icon: typeof props.icon === 'string' ? (ICONS[props.icon] ?? props.icon) : props.icon }),
}

export const vuetify = createVuetify({
  icons: { defaultSet: 'mdi', aliases, sets: { mdi: mdiNamed } },
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
    // Inset switches show their track, so "off" reads as a switch and not a stray dot.
    VSwitch: { inset: true, color: 'primary' },
    VAutocomplete: { variant: 'outlined', density: 'comfortable', rounded: 'lg' },
    VCombobox: { variant: 'outlined', density: 'comfortable', rounded: 'lg' },
    VTextarea: { variant: 'outlined', rounded: 'lg' },
  },
})
