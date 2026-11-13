import { createApp } from 'vue'
import { createPinia } from 'pinia'
import 'vuetify/styles'
import App from './App.vue'
import { router } from './router'
import { vuetify } from './plugins/vuetify'
import { locale } from './i18n'

document.documentElement.lang = locale

createApp(App).use(createPinia()).use(router).use(vuetify).mount('#app')
