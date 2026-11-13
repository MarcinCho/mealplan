<script setup lang="ts">
import BrandLogo from '../components/BrandLogo.vue'
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSession } from '../stores/session'
import { errorText, t } from '../messages'

const session = useSession()
const router = useRouter()
const form = reactive({ householdName: '', displayName: '', login: '', password: '' })
const error = ref('')
const busy = ref(false)

async function submit() {
  busy.value = true
  error.value = ''
  try {
    await session.setup({ ...form })
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
    <v-card width="440" max-width="92vw" class="pa-4" elevation="2">
      <v-card-title tag="h1">{{ t.setupTitle }}</v-card-title>
      <v-card-subtitle class="mb-4">{{ t.setupIntro }}</v-card-subtitle>
      <v-form @submit.prevent="submit">
        <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
        <v-text-field v-model="form.householdName" :label="t.householdName" autofocus />
        <v-text-field v-model="form.displayName" :label="t.displayName" />
        <v-text-field v-model="form.login" :label="t.login" :hint="t.loginHint" persistent-hint autocomplete="username" />
        <v-text-field v-model="form.password" :label="t.password" :hint="t.passwordHint" persistent-hint type="password" autocomplete="new-password" class="mb-4" />
        <v-btn type="submit" color="primary" block :loading="busy">{{ t.createAccount }}</v-btn>
      </v-form>
    </v-card>
  </v-main>
</template>
