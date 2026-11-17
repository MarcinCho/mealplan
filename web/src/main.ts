import { createApp } from 'vue'
import { createPinia } from 'pinia'
import 'vuetify/styles'
import '@fontsource/red-hat-text/latin-400.css'
import '@fontsource/red-hat-text/latin-500.css'
import '@fontsource/red-hat-text/latin-700.css'
import '@fontsource/red-hat-text/latin-ext-400.css'
import '@fontsource/red-hat-text/latin-ext-500.css'
import '@fontsource/red-hat-text/latin-ext-700.css'
import '@fontsource/red-hat-display/latin-600.css'
import '@fontsource/red-hat-display/latin-700.css'
import '@fontsource/red-hat-display/latin-ext-600.css'
import '@fontsource/red-hat-display/latin-ext-700.css'
import './styles/notebook.css'
import App from './App.vue'
import { router } from './router'
import { vuetify } from './plugins/vuetify'
import { locale } from './i18n'

document.documentElement.lang = locale

createApp(App).use(createPinia()).use(router).use(vuetify).mount('#app')
