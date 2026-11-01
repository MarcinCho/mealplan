<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
import { useSession } from '../stores/session'
import { errorText, t } from '../messages'

interface Member { id: string; login: string; displayName: string; role: 'admin' | 'member' }
interface Invite { id: string; role: 'admin' | 'member'; expiresAt: string }

const session = useSession()
const members = ref<Member[]>([])
const invites = ref<Invite[]>([])
const newRole = ref<'admin' | 'member'>('member')
const link = ref('')
const copied = ref(false)
const error = ref('')

const roles = [
  { title: t.member, value: 'member' },
  { title: t.admin, value: 'admin' },
]

async function load() {
  members.value = await api<Member[]>('GET', '/household/members')
  if (session.isAdmin) invites.value = await api<Invite[]>('GET', '/auth/invites')
}

async function create() {
  error.value = ''
  copied.value = false
  try {
    const res = await api<{ path: string }>('POST', '/auth/invites', { role: newRole.value })
    link.value = location.origin + res.path
    await load()
  } catch (e) {
    error.value = errorText(e)
  }
}

async function revoke(id: string) {
  await api('DELETE', `/auth/invites/${id}`)
  await load()
}

async function copy() {
  await navigator.clipboard.writeText(link.value)
  copied.value = true
}

onMounted(load)
</script>

<template>
  <h1 class="text-h5 mb-4">{{ t.members }}</h1>
  <v-list lines="one" border rounded class="mb-8">
    <v-list-item v-for="m in members" :key="m.id" :title="m.displayName" :subtitle="m.login">
      <template #append><v-chip size="small">{{ m.role === 'admin' ? t.admin : t.member }}</v-chip></template>
    </v-list-item>
  </v-list>

  <template v-if="session.isAdmin">
    <h2 class="text-h6 mb-2">{{ t.invites }}</h2>
    <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
    <div class="d-flex flex-wrap ga-3 align-start mb-4">
      <v-select v-model="newRole" :items="roles" :label="t.role" style="max-width: 220px" hide-details />
      <v-btn color="primary" class="mt-1" @click="create">{{ t.createInvite }}</v-btn>
    </div>
    <v-alert v-if="link" type="info" variant="tonal" class="mb-4">
      <div class="mb-2">{{ t.inviteLinkOnce }}</div>
      <code style="word-break: break-all">{{ link }}</code>
      <div class="mt-2"><v-btn size="small" @click="copy">{{ copied ? t.copied : t.copy }}</v-btn></div>
    </v-alert>
    <p v-if="!invites.length" class="text-medium-emphasis">{{ t.noInvites }}</p>
    <v-list v-else lines="one" border rounded>
      <v-list-item v-for="i in invites" :key="i.id" :title="i.role === 'admin' ? t.admin : t.member" :subtitle="`${t.expires} ${new Date(i.expiresAt).toLocaleString('pl-PL')}`">
        <template #append><v-btn size="small" variant="text" color="error" @click="revoke(i.id)">{{ t.revoke }}</v-btn></template>
      </v-list-item>
    </v-list>
  </template>
</template>
