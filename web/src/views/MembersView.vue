<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'
import { copyText } from '../clipboard'
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
  try {
    members.value = await api<Member[]>('GET', '/household/members')
    if (session.isAdmin) invites.value = await api<Invite[]>('GET', '/auth/invites')
  } catch (e) {
    error.value = errorText(e)
  }
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
  error.value = ''
  try {
    await api('DELETE', `/auth/invites/${encodeURIComponent(id)}`)
  } catch (e) {
    error.value = errorText(e)
    return
  }
  await load()
}

async function changeRole(m: Member, role: 'admin' | 'member') {
  if (role === m.role) return
  error.value = ''
  try {
    await api('PATCH', `/household/members/${encodeURIComponent(m.id)}`, { role })
    // Demoting yourself changes your own role: refresh the session so admin-only calls and controls go away.
    if (m.id === session.me?.user.id) await session.fetchMe()
  } catch (e) {
    error.value = errorText(e)
  }
  await load()
}

const removing = ref('')
async function removeMember(id: string) {
  error.value = ''
  try {
    await api('DELETE', `/household/members/${encodeURIComponent(id)}`)
  } catch (e) {
    error.value = errorText(e)
  } finally {
    removing.value = ''
  }
  await load()
}

const adminCount = computed(() => members.value.filter((m) => m.role === 'admin').length)
const isLastAdmin = (m: Member) => m.role === 'admin' && adminCount.value <= 1

async function copy() {
  error.value = ''
  copied.value = await copyText(link.value)
  if (!copied.value) error.value = t.copyFailed
}

onMounted(load)
</script>

<template>
  <h1 class="text-h5 mb-4">{{ t.members }}</h1>
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
  <v-list lines="one" border rounded class="mb-8">
    <v-list-item v-for="m in members" :key="m.id" :title="m.displayName" :subtitle="m.login">
      <template #append>
        <template v-if="session.isAdmin">
          <v-chip v-if="m.id === session.me?.user.id" size="small" class="mr-2">{{ t.you }}</v-chip>
          <v-select
            :model-value="m.role"
            :items="roles"
            :aria-label="t.changeRole"
            :disabled="isLastAdmin(m)"
            density="compact"
            hide-details
            style="max-width: 170px"
            @update:model-value="changeRole(m, $event)"
          />
          <v-btn
            v-if="m.id !== session.me?.user.id && !isLastAdmin(m)"
            class="ml-2"
            size="small"
            variant="text"
            color="error"
            :title="t.removeMember"
            @click="removing === m.id ? removeMember(m.id) : (removing = m.id)"
          >{{ removing === m.id ? t.confirmRemoveMember : t.removeMember }}</v-btn>
        </template>
        <template v-else>
          <v-chip v-if="m.id === session.me?.user.id" size="small" class="mr-2">{{ t.you }}</v-chip>
          <v-chip size="small">{{ m.role === 'admin' ? t.admin : t.member }}</v-chip>
        </template>
      </template>
    </v-list-item>
  </v-list>

  <template v-if="session.isAdmin">
    <h2 class="text-h6 mb-2">{{ t.invites }}</h2>
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
