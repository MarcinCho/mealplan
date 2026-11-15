<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'
import AssistantSettings from '../components/AssistantSettings.vue'
import { useMealPrepStore } from '../stores/mealprep'
import { useSession } from '../stores/session'
import { errorText, t } from '../messages'

// Household-wide settings in one place: the name (administrator), the default meal prep portions
// (any member) and the product assistant (administrator).
const session = useSession()
const prep = useMealPrepStore()
const error = ref('')

const name = ref('')
const nameSaved = ref(false)
const nameOk = computed(() => {
  const n = [...name.value.trim()].length
  return n >= 1 && n <= 100
})

const portions = ref<number | string>(1)
const portionsSaved = ref(false)
const portionsOk = computed(() => Number.isInteger(Number(portions.value)) && Number(portions.value) >= 1 && Number(portions.value) <= 20)

async function load() {
  try {
    const res = await api<{ name: string; portionsPerMeal: number }>('GET', '/household/settings')
    name.value = res.name
    portions.value = res.portionsPerMeal
    prep.portionsPerMeal = res.portionsPerMeal
  } catch (e) {
    error.value = errorText(e)
  }
}

async function saveName() {
  if (!nameOk.value) return
  error.value = ''
  nameSaved.value = false
  try {
    const res = await api<{ name: string }>('PUT', '/household/settings', { name: name.value })
    name.value = res.name
    nameSaved.value = true
    // The app bar and other screens read the name from the session.
    await session.fetchMe()
  } catch (e) {
    error.value = errorText(e)
  }
}

async function savePortions() {
  if (!portionsOk.value) return
  error.value = ''
  portionsSaved.value = false
  try {
    await prep.savePortionsPerMeal(Number(portions.value))
    portionsSaved.value = true
  } catch (e) {
    error.value = errorText(e)
  }
}

onMounted(load)
</script>

<template>
  <h1 class="text-h5 mb-4">{{ t.householdSettings }}</h1>
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>

  <section class="mb-8" style="max-width: 560px">
    <h2 class="text-h6 mb-2">{{ t.householdName }}</h2>
    <div v-if="session.isAdmin" class="d-flex flex-wrap ga-3 align-start">
      <v-text-field
        v-model="name"
        :label="t.householdName"
        maxlength="100"
        class="flex-grow-1"
        style="min-width: 220px"
        :error="!nameOk"
        @update:model-value="nameSaved = false"
        @keydown.enter="saveName"
      />
      <v-btn color="primary" class="mt-1" :disabled="!nameOk" @click="saveName">{{ nameSaved ? t.settingSaved : t.save }}</v-btn>
    </div>
    <template v-else>
      <p class="text-body-1" data-test="household-name">{{ name }}</p>
      <p class="text-body-2 text-medium-emphasis">{{ t.householdNameAdminOnly }}</p>
    </template>
  </section>

  <section class="mb-8">
    <h2 class="text-h6 mb-2">{{ t.portionsPerMeal }}</h2>
    <div class="d-flex flex-wrap ga-3 align-start">
      <v-text-field
        v-model="portions"
        type="number"
        min="1"
        max="20"
        :label="t.portionsPerMeal"
        :hint="t.portionsPerMealHint"
        persistent-hint
        style="max-width: 260px"
        :error="!portionsOk"
        @update:model-value="portionsSaved = false"
        @keydown.enter="savePortions"
      />
      <v-btn color="primary" class="mt-1" :disabled="!portionsOk" @click="savePortions">{{ portionsSaved ? t.settingSaved : t.save }}</v-btn>
    </div>
  </section>

  <AssistantSettings />
</template>
