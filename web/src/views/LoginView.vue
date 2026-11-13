<script setup lang="ts">
import BrandLogo from '../components/BrandLogo.vue'
import LanguageSwitch from '../components/LanguageSwitch.vue'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSession } from '../stores/session'
import { errorText, t } from '../messages'

const session = useSession()
const router = useRouter()
const login = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  busy.value = true
  error.value = ''
  try {
    await session.login(login.value, password.value)
    await router.push({ name: 'home' })
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <v-main class="d-flex flex-column align-center justify-center">
    <BrandLogo :height="50" class="mb-6" style="max-width: 70vw" />
    <v-card width="400" max-width="92vw" class="pa-4" elevation="2">
      <v-form @submit.prevent="submit">
        <h1 class="text-h6 mb-4">{{ t.loginTitle }}</h1>
        <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
        <v-text-field v-model="login" :label="t.login" autocomplete="username" autofocus />
        <v-text-field v-model="password" :label="t.password" type="password" autocomplete="current-password" />
        <v-btn type="submit" color="primary" size="large" block :loading="busy">{{ t.signIn }}</v-btn>
      </v-form>
    </v-card>
    <LanguageSwitch class="mt-6" />
  </v-main>
</template>
