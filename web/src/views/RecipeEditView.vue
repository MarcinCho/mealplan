<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { draftToRecipe, mergeIntoExisting } from '../importDraft'
import { useImport } from '../stores/import'
import { emptyRecipe, useRecipes } from '../stores/recipes'
import type { Macros, Recipe } from '../api/types'
import ProductPicker from '../components/ProductPicker.vue'
import { formatNumber } from '../nutrition'
import { errorText, t } from '../messages'

const route = useRoute()
const router = useRouter()
const recipes = useRecipes()
const imports = useImport()
const recipe = ref<Recipe>(emptyRecipe())
const error = ref('')
const busy = ref(false)
const confirming = ref(false)
const loaded = ref(false)
const savedOk = ref(false)
const isNew = computed(() => !route.params.id)

const FIELDS: { key: keyof Macros; label: string; unit: 'kcal' | 'g' }[] = [
  { key: 'kcal', label: t.kcalShort, unit: 'kcal' },
  { key: 'protein', label: t.proteinShort, unit: 'g' },
  { key: 'carbs', label: t.carbsShort, unit: 'g' },
  { key: 'fat', label: t.fatShort, unit: 'g' },
]

function normalize(r: Recipe): Recipe {
  return {
    ...r,
    ingredients: r.ingredients.map((i) => ({ ...i, note: i.note ?? '', productId: i.productId ?? null })),
    nutrition: { kcal: null, protein: null, carbs: null, fat: null, ...(r.nutrition ?? {}) },
  }
}

// Hint for an empty field: the computed value, or that there is none.
function hint(k: keyof Macros, unit: 'kcal' | 'g'): string {
  const np = recipe.value.nutritionPerPortion
  const v = np?.values[k]
  if (!np || v === null || v === undefined || np.sources[k] === 'missing') return t.nutritionMissing
  return `${formatNumber(v, unit)} (${t.nutritionComputed})`
}

onMounted(async () => {
  const d = imports.draft
  if (isNew.value) {
    if (d && !imports.targetId) recipe.value = normalize(draftToRecipe(d))
    loaded.value = true
    return
  }
  try {
    const existing = await recipes.get(String(route.params.id))
    if (d && imports.targetId === existing.id) {
      recipe.value = normalize({ ...mergeIntoExisting(d, existing), nutritionPerPortion: existing.nutritionPerPortion })
    } else {
      recipe.value = normalize(existing)
    }
    loaded.value = true
  } catch (e) {
    error.value = errorText(e)
  }
})

// A draft must never leak into the next "new recipe" form.
onBeforeUnmount(() => imports.clear())

const fromImport = computed(() => Boolean(imports.draft) && (isNew.value ? !imports.targetId : imports.targetId === recipe.value.id))

function addIngredient() {
  recipe.value.ingredients.push({ name: '', quantity: null, unit: '', note: '', productId: null })
}
function addStep() {
  recipe.value.steps.push('')
}

async function submit() {
  if (!loaded.value || (!isNew.value && !recipe.value.id)) return
  error.value = ''
  savedOk.value = false
  if (!recipe.value.title.trim()) {
    error.value = t.titleRequired
    return
  }
  busy.value = true
  try {
    const saved = await recipes.save(recipe.value)
    imports.clear()
    if (isNew.value) {
      // The edit route reloads the recipe, including the freshly computed values.
      await router.replace({ name: 'recipe-edit', params: { id: saved.id } })
    } else {
      recipe.value = normalize(await recipes.get(saved.id))
      savedOk.value = true
    }
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}

async function remove() {
  if (!loaded.value || isNew.value || !recipe.value.id) return
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
  <v-alert v-if="savedOk" type="success" variant="tonal" class="mb-4" closable @click:close="savedOk = false">{{ t.saved }}</v-alert>
  <v-alert v-if="loaded && fromImport" type="info" variant="tonal" class="mb-4">
    {{ t.importedBanner }}
    <div v-if="!isNew" class="mt-2">{{ t.importedUpdateNote }}</div>
    <ul v-if="imports.draft?.warnings.length" class="mt-2 ps-4">
      <li v-for="(w, wi) in imports.draft.warnings" :key="wi">{{ w }}</li>
    </ul>
  </v-alert>
  <v-progress-linear v-if="!loaded && !error" indeterminate class="mb-4" :aria-label="t.loading" />
  <v-btn v-if="!loaded && error" variant="text" :to="{ name: 'recipes' }">{{ t.backToList }}</v-btn>
  <v-form v-if="loaded" @submit.prevent="submit">
    <v-text-field v-model="recipe.title" :label="t.recipeTitle" maxlength="200" required />
    <div class="d-flex ga-3 flex-wrap">
      <v-text-field v-model.number="recipe.baseServings" :label="t.baseServings" type="number" min="1" max="100" style="max-width: 180px" />
      <v-text-field v-model="recipe.sourceUrl" :label="t.sourceUrl" class="flex-grow-1" />
    </div>
    <v-combobox v-model="recipe.tags" :label="t.tags" multiple chips closable-chips />

    <h2 class="text-h6 mb-1">{{ t.nutritionPerPortion }}</h2>
    <div class="text-caption text-medium-emphasis mb-2">{{ t.nutritionPerPortionHint }}</div>
    <v-alert v-if="recipe.nutritionPerPortion?.unmatched.length" type="warning" variant="tonal" density="compact" class="mb-3">
      {{ t.nutritionApproxPrefix }} {{ recipe.nutritionPerPortion.unmatched.join(', ') }}
    </v-alert>
    <div class="nutri-grid mb-2">
      <v-text-field
        v-for="f in FIELDS"
        :key="f.key"
        v-model.number="recipe.nutrition![f.key]"
        :label="f.label"
        :placeholder="hint(f.key, f.unit)"
        persistent-placeholder
        type="number"
        min="0"
        step="any"
        density="compact"
      />
    </div>

    <h2 class="text-h6 mb-2">{{ t.ingredients }}</h2>
    <div v-for="(ing, i) in recipe.ingredients" :key="i" :class="['ing-row', ing.confidence && `ing-${ing.confidence}`]">
      <div class="d-flex ga-2 align-start ing-fields">
        <v-text-field v-model="ing.name" :label="t.ingredientName" class="flex-grow-1 ing-name" density="compact" />
        <v-text-field v-model.number="ing.quantity" :label="t.quantity" type="number" min="0.001" step="any" density="compact" class="ing-qty" />
        <v-text-field v-model="ing.unit" :label="t.unit" density="compact" class="ing-unit" />
        <v-btn icon="mdi-close" variant="text" :title="t.remove" @click="recipe.ingredients.splice(i, 1)" />
      </div>
      <ProductPicker :matched-id="ing.matchedProductId ?? null" :model-value="ing.productId ?? null" @update:model-value="ing.productId = $event" />
      <v-text-field v-model="ing.note" :label="t.ingredientNote" :placeholder="t.ingredientNotePlaceholder" maxlength="200" density="compact" />
      <div v-if="ing.confidence && ing.confidence !== 'ok'" class="text-caption text-medium-emphasis pb-1">{{ t.fromText }}: „{{ ing.raw }}"</div>
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

<style scoped>
.ing-row {
  border-radius: 12px;
  padding: 0 8px;
}
.ing-guess,
.ing-unparsed {
  margin-bottom: 8px;
}
.ing-guess {
  background: rgba(var(--v-theme-warning), 0.16);
  border-left: 3px solid rgb(var(--v-theme-warning));
}
.ing-unparsed {
  background: rgba(var(--v-theme-error), 0.16);
  border-left: 3px solid rgb(var(--v-theme-error));
}
:global(.v-theme--dark) .ing-unparsed {
  background: rgba(var(--v-theme-error), 0.24);
}
.ing-qty,
.ing-unit {
  max-width: 110px;
}
.nutri-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}
@media (max-width: 599.98px) {
  .nutri-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 599.98px) {
  .ing-fields {
    flex-wrap: wrap;
  }
  .ing-name {
    flex: 1 1 100%;
  }
  .ing-qty,
  .ing-unit {
    flex: 1 1 0;
    max-width: none;
  }
}
</style>
