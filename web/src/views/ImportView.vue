<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { onBeforeRouteLeave, useRouter } from 'vue-router'
import { findDuplicate, importErrorText, keepsDraft, normalizeImportUrl } from '../importDraft'
import { useImport } from '../stores/import'
import { useRecipes } from '../stores/recipes'
import type { Recipe, RecipeDraft } from '../api/types'
import { t } from '../messages'

const router = useRouter()
const imports = useImport()
const recipes = useRecipes()
const tab = ref<'url' | 'text'>('url')
const url = ref('')
const text = ref('')
const busy = ref(false)
const error = ref('')
const duplicate = ref<Recipe | null>(null)
// While the duplicate card is shown the draft lives here, so the store (and the nav's "new recipe") never sees it.
const pending = ref<RecipeDraft | null>(null)

// Bumped on every new request and when the user leaves; a response from an older request must change nothing.
let request = 0

onMounted(() => {
  imports.clear()
})

onBeforeRouteLeave((to) => {
  request++
  if (!keepsDraft(to.name)) imports.clear()
})

onBeforeUnmount(() => {
  request++
})

function back() {
  duplicate.value = null
  pending.value = null
  imports.clear()
}

async function analyze() {
  if (busy.value) return
  error.value = ''
  duplicate.value = null
  pending.value = null
  busy.value = true
  const mine = ++request
  const stale = () => mine !== request
  try {
    const draft = tab.value === 'url' ? await imports.fetchUrl(normalizeImportUrl(url.value)) : await imports.fetchText(text.value)
    if (stale()) return
    try {
      if (!recipes.loaded) await recipes.load()
      if (stale()) return
      const dup = findDuplicate(draft, recipes.items)
      if (dup) {
        pending.value = draft
        duplicate.value = dup
        return
      }
    } catch {
      // The duplicate check is best effort; a good draft must not be lost because the list failed to load.
    }
    imports.setDraft(draft)
    await router.push({ name: 'recipe-new' })
  } catch (e) {
    if (mine !== request) return
    error.value = importErrorText(e, tab.value)
  } finally {
    busy.value = false
  }
}

async function choose(update: boolean) {
  const dup = duplicate.value
  if (pending.value) imports.setDraft(pending.value)
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
      <v-btn variant="text" @click="back">{{ t.back }}</v-btn>
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

  <v-card v-if="!duplicate" border flat class="pa-4 mt-6">
    <div class="text-subtitle-1 mb-1">{{ t.skillTitle }}</div>
    <div class="text-body-2 mb-3">{{ t.skillText }}</div>
    <v-btn variant="tonal" prepend-icon="mdi-download" href="/claude-skill/SKILL.md" download="SKILL.md">{{ t.skillDownload }}</v-btn>
    <div class="text-caption text-medium-emphasis mt-3">{{ t.skillInstall }}</div>
  </v-card>
</template>
