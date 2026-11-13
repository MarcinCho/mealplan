<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { errorText, t } from '../messages'
import LanguageSwitch from '../components/LanguageSwitch.vue'
import { useNutrition } from '../stores/nutrition'
import type { Macros } from '../api/types'

const nutrition = useNutrition()
const enabled = ref(false)
const goal = ref<Macros>({ kcal: null, protein: null, carbs: null, fat: null })
const error = ref('')
const saved = ref(false)
const busy = ref(false)

const fields: { key: keyof Macros; label: string; max: number }[] = [
  { key: 'kcal', label: t.goalKcal, max: 20000 },
  { key: 'protein', label: t.goalProtein, max: 2000 },
  { key: 'carbs', label: t.goalCarbs, max: 2000 },
  { key: 'fat', label: t.goalFat, max: 2000 },
]

function fill() {
  if (!nutrition.settings) return
  enabled.value = nutrition.settings.enabled
  goal.value = { ...nutrition.settings.goal }
}

async function save() {
  error.value = ''
  saved.value = false
  busy.value = true
  try {
    await nutrition.saveSettings({ enabled: enabled.value, goal: goal.value })
    fill()
    saved.value = true
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  try {
    await nutrition.loadSettings()
    fill()
  } catch (e) {
    error.value = errorText(e)
  }
})
</script>

<template>
  <h1 class="text-h5 mb-4">{{ t.accountTitle }}</h1>
  <LanguageSwitch class="mb-4" />
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
  <v-alert v-if="saved" type="success" variant="tonal" class="mb-4">{{ t.saved }}</v-alert>
  <v-card variant="outlined" class="pa-4" style="max-width: 560px">
    <v-switch v-model="enabled" :label="t.trackNutrition" :hint="t.trackNutritionHint" persistent-hint color="primary" />
    <template v-if="enabled">
      <h2 class="text-subtitle-1 mt-6">{{ t.dailyGoal }}</h2>
      <p class="text-body-2 text-medium-emphasis mb-3">{{ t.dailyGoalHint }}</p>
      <v-row dense>
        <v-col v-for="f in fields" :key="f.key" cols="12" sm="6">
          <v-text-field v-model.number="goal[f.key]" :label="f.label" type="number" min="0" :max="f.max" inputmode="decimal" />
        </v-col>
      </v-row>
    </template>
    <div class="mt-4">
      <v-btn color="primary" :loading="busy" @click="save">{{ t.save }}</v-btn>
    </div>
  </v-card>
</template>
