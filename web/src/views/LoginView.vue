<script setup lang="ts">
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
  <v-main class="d-flex align-center justify-center">
    <v-card width="400" max-width="92vw" class="pa-4" elevation="2">
      <v-card-title>{{ t.appName }}</v-card-title>
      <v-form @submit.prevent="submit">
        <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
        <v-text-field v-model="login" :label="t.login" autocomplete="username" autofocus />
        <v-text-field v-model="password" :label="t.password" type="password" autocomplete="current-password" />
        <v-btn type="submit" color="primary" block :loading="busy">{{ t.signIn }}</v-btn>
      </v-form>
    </v-card>
  </v-main>
</template>
