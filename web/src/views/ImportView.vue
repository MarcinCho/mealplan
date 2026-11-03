<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { findDuplicate } from '../importDraft'
import { useImport } from '../stores/import'
import { useRecipes } from '../stores/recipes'
import type { Recipe } from '../api/types'
import { errorText, t } from '../messages'

const router = useRouter()
const imports = useImport()
const recipes = useRecipes()
const tab = ref<'url' | 'text'>('url')
const url = ref('')
const text = ref('')
const busy = ref(false)
const error = ref('')
const duplicate = ref<Recipe | null>(null)

onMounted(() => {
  imports.clear()
})

async function analyze() {
  error.value = ''
  duplicate.value = null
  busy.value = true
  try {
    const draft = tab.value === 'url' ? await imports.fromUrl(url.value) : await imports.fromText(text.value)
    if (!recipes.loaded) await recipes.load()
    const dup = findDuplicate(draft, recipes.items)
    if (dup) {
      duplicate.value = dup
      return
    }
    await router.push({ name: 'recipe-new' })
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}

async function choose(update: boolean) {
  const dup = duplicate.value
  if (update && dup) {
    imports.targetId = dup.id
    await router.push({ name: 'recipe-edit', params: { id: dup.id } })
  } else {
    imports.targetId = ''
    await router.push({ name: 'recipe-new' })
  }
}
</script>

<template>
  <h1 class="text-h5 mb-4">{{ t.importTitle }}</h1>
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>

  <v-card v-if="duplicate" border flat class="pa-4 mb-4">
    <div class="text-subtitle-1 mb-1">{{ t.importDuplicateTitle }}</div>
    <div class="text-body-2 mb-3">{{ t.importDuplicateHint }} <strong>{{ duplicate.title }}</strong></div>
    <div class="d-flex ga-3 flex-wrap">
      <v-btn color="primary" @click="choose(true)">{{ t.importUpdate }}</v-btn>
      <v-btn variant="tonal" @click="choose(false)">{{ t.importAddNew }}</v-btn>
    </div>
  </v-card>

  <v-form v-else @submit.prevent="analyze">
    <v-tabs v-model="tab" class="mb-4">
      <v-tab value="url">{{ t.importLink }}</v-tab>
      <v-tab value="text">{{ t.importText }}</v-tab>
    </v-tabs>
    <v-text-field v-if="tab === 'url'" v-model="url" :label="t.importUrlLabel" :hint="t.importUrlHint" persistent-hint type="url" autofocus />
    <v-textarea v-else v-model="text" :label="t.importTextLabel" :hint="t.importTextHint" persistent-hint rows="12" auto-grow autofocus />
    <div class="d-flex ga-3 mt-4">
      <v-btn type="submit" color="primary" :loading="busy" :disabled="tab === 'url' ? !url.trim() : !text.trim()">{{ t.analyze }}</v-btn>
      <v-btn variant="text" :to="{ name: 'recipes' }">{{ t.cancel }}</v-btn>
    </div>
  </v-form>
</template>
