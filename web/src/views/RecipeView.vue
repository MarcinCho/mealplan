<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ApiError } from '../api/client'
import type { Macros, Recipe } from '../api/types'
import { useRecipes, formatQuantity } from '../stores/recipes'
import { useNutrition } from '../stores/nutrition'
import { formatNumber } from '../nutrition'
import { MAX_SERVINGS, parseServings, scaleAmount } from '../scale'
import { errorText, t } from '../messages'

const route = useRoute()
const router = useRouter()
const recipes = useRecipes()
const nutrition = useNutrition()

const recipe = ref<Recipe | null>(null)
const error = ref('')

const id = computed(() => String(route.params.id))
const base = computed(() => recipe.value?.baseServings ?? 1)
const servings = computed(() => parseServings(route.query.servings, base.value))
const scaled = computed(() => servings.value !== base.value)

function setServings(n: number) {
  const query = { ...route.query }
  if (n === base.value) delete query.servings
  else query.servings = String(n)
  router.replace({ query })
}

const ingredients = computed(() => {
  const factor = servings.value / base.value
  return (recipe.value?.ingredients ?? []).map((i) => {
    const a = scaleAmount(i.quantity, i.unit, factor)
    return { amount: formatQuantity(a.quantity, a.unit), name: i.name, note: i.note ?? '' }
  })
})

const nutritionLine = computed(() => {
  const n = recipe.value?.nutritionPerPortion
  if (nutrition.settings?.enabled !== true || !n) return ''
  const v = n.values
  const known = (k: keyof Macros) => v[k] !== null && v[k] !== undefined
  const parts: string[] = []
  if (known('kcal')) {
    parts.push(`${n.approximate ? t.approxPrefix + ' ' : ''}${formatNumber(v.kcal, 'kcal')} kcal ${t.perPortion}`)
  }
  for (const k of ['protein', 'carbs', 'fat'] as const) {
    if (known(k)) parts.push(`${formatNumber(v[k], 'g')} g ${t.gramsOf[k]}`)
  }
  return parts.join(' · ')
})

const sourceHost = computed(() => {
  const url = recipe.value?.sourceUrl ?? ''
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return url
  }
})

onMounted(async () => {
  if (nutrition.settings === null) nutrition.loadSettings().catch(() => {})
  try {
    recipe.value = await recipes.get(id.value)
  } catch (e) {
    error.value = e instanceof ApiError && e.status === 404 ? t.recipeNotFound : errorText(e)
  }
})
</script>

<template>
  <div class="recipe-view">
    <v-btn variant="text" :to="{ name: 'recipes' }" prepend-icon="mdi-chevron-left" class="mb-2">{{ t.recipes }}</v-btn>
    <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
    <v-progress-linear v-else-if="!recipe" indeterminate class="mb-4" :aria-label="t.loading" />
    <template v-if="recipe">
      <div class="d-flex align-start ga-2">
        <h1 class="text-h5 flex-grow-1 title">{{ recipe.title }}</h1>
        <v-btn variant="tonal" :to="{ name: 'recipe-edit', params: { id: recipe.id } }">{{ t.editButton }}</v-btn>
      </div>
      <div v-if="recipe.tags.length" class="d-flex flex-wrap ga-1 mt-1">
        <v-chip v-for="tag in recipe.tags" :key="tag" size="small">{{ tag }}</v-chip>
      </div>
      <p v-if="nutritionLine" class="text-body-2 text-medium-emphasis mt-2 nutrition">{{ nutritionLine }}</p>

      <div class="d-flex align-center ga-2 mt-4">
        <span class="text-subtitle-1">{{ t.servingsLabel }}</span>
        <v-btn
          icon="mdi-minus"
          variant="tonal"
          class="step-btn"
          :aria-label="t.fewerServings"
          :disabled="servings <= 1"
          @click="setServings(servings - 1)"
        />
        <span class="text-h6 servings" aria-live="polite">{{ servings }}</span>
        <v-btn
          icon="mdi-plus"
          variant="tonal"
          class="step-btn"
          :aria-label="t.moreServings"
          :disabled="servings >= MAX_SERVINGS"
          @click="setServings(servings + 1)"
        />
      </div>
      <div v-if="scaled" class="text-body-2 text-medium-emphasis mt-1">
        {{ t.scaledFrom(base) }} ·
        <v-btn variant="text" size="small" class="restore" @click="setServings(base)">{{ t.restoreServings }}</v-btn>
      </div>

      <h2 class="text-h6 mt-6 mb-2">{{ t.ingredients }}</h2>
      <ul class="list-plain">
        <li v-for="(i, n) in ingredients" :key="n" class="ingredient py-1">
          <span>{{ i.amount ? `${i.amount} ${i.name}` : i.name }}</span>
          <span v-if="i.note" class="text-medium-emphasis ms-2">{{ i.note }}</span>
        </li>
      </ul>

      <template v-if="recipe.steps.length">
        <h2 class="text-h6 mt-6 mb-2">{{ t.steps }}</h2>
        <ol class="steps ps-6">
          <li v-for="(s, n) in recipe.steps" :key="n" class="py-1">{{ s }}</li>
        </ol>
      </template>

      <p v-if="recipe.sourceUrl" class="mt-6">
        {{ t.source }}:
        <a :href="recipe.sourceUrl" target="_blank" rel="noopener noreferrer">{{ sourceHost }}</a>
      </p>
    </template>
  </div>
</template>

<style scoped>
.recipe-view { max-width: 720px; margin: 0 auto; }
.title { overflow-wrap: anywhere; }
.list-plain { list-style: none; padding: 0; }
.step-btn { min-width: 44px; min-height: 44px; }
.servings { min-width: 2ch; text-align: center; }
</style>
