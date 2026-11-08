<script setup lang="ts">
import BrandLogo from '../components/BrandLogo.vue'
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { useSession } from '../stores/session'
import { errorText, t } from '../messages'

const route = useRoute()
const router = useRouter()
const session = useSession()
const token = String(route.params.token)
const householdName = ref('')
const valid = ref<boolean | null>(null)
const form = reactive({ displayName: '', login: '', password: '' })
const error = ref('')
const busy = ref(false)

onMounted(async () => {
  try {
    const info = await api<{ householdName: string }>('GET', `/auth/invitations/${encodeURIComponent(token)}`)
    householdName.value = info.householdName
    valid.value = true
  } catch {
    valid.value = false
  }
})

async function submit() {
  busy.value = true
  error.value = ''
  try {
    await session.acceptInvite(token, { ...form })
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
      <v-card-title>{{ t.inviteTitle }}</v-card-title>
      <v-alert v-if="valid === false" type="error" variant="tonal" class="mt-2">{{ t.inviteInvalid }}</v-alert>
      <v-form v-else-if="valid" @submit.prevent="submit">
        <v-card-subtitle class="mb-4">{{ householdName }}</v-card-subtitle>
        <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
        <v-text-field v-model="form.displayName" :label="t.displayName" autofocus />
        <v-text-field v-model="form.login" :label="t.login" :hint="t.loginHint" persistent-hint autocomplete="username" />
        <v-text-field v-model="form.password" :label="t.password" :hint="t.passwordHint" persistent-hint type="password" autocomplete="new-password" class="mb-4" />
        <v-btn type="submit" color="primary" block :loading="busy">{{ t.createAccount }}</v-btn>
      </v-form>
    </v-card>
  </v-main>
</template>
