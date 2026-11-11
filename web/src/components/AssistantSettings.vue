<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useAssistant } from '../stores/assistant'
import { useSession } from '../stores/session'
import { errorText, t } from '../messages'

const assistant = useAssistant()
const session = useSession()
// Never filled from the server: only what the admin types.
const key = ref('')
const model = ref('')
const error = ref('')
const status = ref<'' | 'saved' | 'ok'>('')
const busy = ref(false)

async function run(fn: () => Promise<void>, done: 'saved' | 'ok' | '') {
  error.value = ''
  status.value = ''
  busy.value = true
  try {
    await fn()
    status.value = done
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}

const save = () =>
  run(async () => {
    await assistant.save(key.value.trim() === '' ? undefined : key.value.trim(), model.value.trim())
    key.value = ''
    model.value = assistant.model
  }, 'saved')
const remove = () => run(() => assistant.remove(), '')
const test = () => run(() => assistant.test(), 'ok')

onMounted(async () => {
  try {
    await assistant.load()
    model.value = assistant.model
  } catch (e) {
    error.value = errorText(e)
  }
})
</script>

<template>
  <section class="mb-8">
    <h2 class="text-h6 mb-2">{{ t.assistantTitle }}</h2>
    <v-alert v-if="error" type="error" variant="tonal" class="mb-3">{{ error }}</v-alert>
    <p v-if="assistant.loaded" class="mb-3" data-test="key-state">
      {{ assistant.configured ? t.assistantKeySet(assistant.keyHint) : t.assistantNoKey }}
    </p>
    <template v-if="session.isAdmin">
      <div class="d-flex flex-wrap ga-3 align-start">
        <v-text-field
          v-model="key"
          type="password"
          autocomplete="off"
          :label="t.assistantKey"
          style="max-width: 360px"
          @keydown.enter="save"
        />
        <v-text-field
          v-model="model"
          :label="t.assistantModel"
          :placeholder="assistant.defaultModel"
          style="max-width: 300px"
          @keydown.enter="save"
        />
      </div>
      <div class="d-flex flex-wrap ga-2">
        <v-btn color="primary" :disabled="busy" @click="save">{{ status === 'saved' ? t.settingSaved : t.save }}</v-btn>
        <v-btn :disabled="busy || !assistant.configured" @click="test">{{ status === 'ok' ? t.assistantOk : t.assistantTest }}</v-btn>
        <v-btn v-if="assistant.configured" variant="text" color="error" :disabled="busy" @click="remove">{{ t.assistantRemoveKey }}</v-btn>
      </div>
    </template>
    <p class="text-medium-emphasis text-body-2 mt-3">{{ t.assistantPrivacy }}</p>
  </section>
</template>
