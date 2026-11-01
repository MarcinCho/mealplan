<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { emptyRecipe, useRecipes } from '../stores/recipes'
import type { Recipe } from '../api/types'
import { errorText, t } from '../messages'

const route = useRoute()
const router = useRouter()
const recipes = useRecipes()
const recipe = ref<Recipe>(emptyRecipe())
const error = ref('')
const busy = ref(false)
const confirming = ref(false)
const isNew = computed(() => !route.params.id)

onMounted(async () => {
  if (isNew.value) return
  try {
    recipe.value = await recipes.get(String(route.params.id))
  } catch (e) {
    error.value = errorText(e)
  }
})

function addIngredient() {
  recipe.value.ingredients.push({ name: '', quantity: null, unit: '' })
}
function addStep() {
  recipe.value.steps.push('')
}

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await recipes.save(recipe.value)
    await router.push({ name: 'recipes' })
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}

async function remove() {
  error.value = ''
  try {
    await recipes.remove(recipe.value.id)
    await router.push({ name: 'recipes' })
  } catch (e) {
    error.value = errorText(e)
    confirming.value = false
  }
}
</script>

<template>
  <h1 class="text-h5 mb-4">{{ isNew ? t.newRecipe : t.editRecipe }}</h1>
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
  <v-form @submit.prevent="submit">
    <v-text-field v-model="recipe.title" :label="t.recipeTitle" maxlength="200" required />
    <div class="d-flex ga-3 flex-wrap">
      <v-text-field v-model.number="recipe.baseServings" :label="t.baseServings" type="number" min="1" max="100" style="max-width: 180px" />
      <v-text-field v-model="recipe.sourceUrl" :label="t.sourceUrl" class="flex-grow-1" />
    </div>
    <v-combobox v-model="recipe.tags" :label="t.tags" multiple chips closable-chips />

    <h2 class="text-h6 mb-2">{{ t.ingredients }}</h2>
    <div v-for="(ing, i) in recipe.ingredients" :key="i" class="d-flex ga-2 align-start">
      <v-text-field v-model="ing.name" :label="t.ingredientName" class="flex-grow-1" density="compact" />
      <v-text-field v-model.number="ing.quantity" :label="t.quantity" type="number" min="0" step="any" density="compact" style="max-width: 110px" />
      <v-text-field v-model="ing.unit" :label="t.unit" density="compact" style="max-width: 110px" />
      <v-btn icon="mdi-close" variant="text" :title="t.remove" @click="recipe.ingredients.splice(i, 1)" />
    </div>
    <v-btn variant="text" prepend-icon="mdi-plus" class="mb-4" @click="addIngredient">{{ t.addIngredient }}</v-btn>

    <h2 class="text-h6 mb-2">{{ t.steps }}</h2>
    <div v-for="(_, i) in recipe.steps" :key="i" class="d-flex ga-2 align-start">
      <v-textarea v-model="recipe.steps[i]" :label="`${i + 1}.`" rows="2" auto-grow density="compact" class="flex-grow-1" />
      <v-btn icon="mdi-close" variant="text" :title="t.remove" @click="recipe.steps.splice(i, 1)" />
    </div>
    <v-btn variant="text" prepend-icon="mdi-plus" class="mb-6" @click="addStep">{{ t.addStep }}</v-btn>

    <div class="d-flex ga-3">
      <v-btn type="submit" color="primary" :loading="busy">{{ t.save }}</v-btn>
      <v-btn variant="text" :to="{ name: 'recipes' }">{{ t.cancel }}</v-btn>
      <v-spacer />
      <template v-if="!isNew">
        <v-btn v-if="!confirming" color="error" variant="text" @click="confirming = true">{{ t.delete }}</v-btn>
        <v-btn v-else color="error" @click="remove">{{ t.confirmDelete }}</v-btn>
      </template>
    </div>
  </v-form>
</template>
