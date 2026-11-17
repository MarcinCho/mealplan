<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useDisplay } from 'vuetify'
import { ApiError } from '../api/client'
import type { Recipe } from '../api/types'
import { useRecipes, formatQuantity } from '../stores/recipes'
import { useNutrition } from '../stores/nutrition'
import { formatMacros } from '../nutrition'
import { MAX_SERVINGS, parseServings, scaleAmount } from '../scale'
import { errorText, t } from '../messages'
import NoteCard from '../components/notebook/NoteCard.vue'

const route = useRoute()
const router = useRouter()
const { mdAndUp } = useDisplay()
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
  if (![v.kcal, v.protein, v.carbs, v.fat].some((x) => x !== null && x !== undefined)) return ''
  // The qualifiers describe the whole line: approximate and per portion, whichever values it shows.
  return `${n.approximate ? t.approxPrefix + ' ' : ''}${formatMacros(v)} ${t.perPortion}`
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
  <div class="recipe-view" :class="{ 'recipe-view--columns': mdAndUp }">
    <v-btn variant="text" :to="{ name: 'recipes' }" prepend-icon="mdi-chevron-left" class="mb-2">{{ t.recipes }}</v-btn>
    <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
    <v-progress-linear v-else-if="!recipe" indeterminate class="mb-4" :aria-label="t.loading" />
    <template v-if="recipe">
      <header class="recipe-head">
        <h1 class="text-h4 title">{{ recipe.title }}</h1>
        <div v-if="recipe.tags.length" class="d-flex flex-wrap ga-1 tags">
          <v-chip v-for="tag in recipe.tags" :key="tag" size="small">{{ tag }}</v-chip>
        </div>
        <div class="recipe-acts">
          <span v-if="recipe.sourceUrl" class="source-line">{{ t.source }}: <a :href="recipe.sourceUrl" target="_blank" rel="noopener noreferrer" class="source">{{ sourceHost }}</a></span>
          <v-btn variant="outlined" prepend-icon="mdi-pencil-outline" :to="{ name: 'recipe-edit', params: { id: recipe.id } }">{{ t.editButton }}</v-btn>
        </div>
        <p v-if="nutritionLine" class="text-body-2 text-medium-emphasis nutrition">{{ nutritionLine }}</p>
      </header>

      <div class="recipe-body">
        <aside class="recipe-view__aside">
          <NoteCard :title="ingredients.length ? t.ingredients : undefined">
            <template #actions>
              <div class="stepper" role="group" aria-labelledby="servings-label">
                <span id="servings-label" class="text-subtitle-2">{{ t.servingsLabel }}</span>
                <v-btn
                  icon="mdi-minus"
                  variant="tonal"
                  size="small"
                  class="step-btn"
                  :aria-label="t.fewerServings"
                  :disabled="servings <= 1"
                  @click="setServings(servings - 1)"
                />
                <span class="text-h6 servings mp-num" aria-live="polite">{{ servings }}</span>
                <v-btn
                  icon="mdi-plus"
                  variant="tonal"
                  size="small"
                  class="step-btn"
                  :aria-label="t.moreServings"
                  :disabled="servings >= MAX_SERVINGS"
                  @click="setServings(servings + 1)"
                />
              </div>
            </template>
            <template v-if="scaled" #subheader>
              <span class="text-body-2 text-medium-emphasis">
                {{ t.scaledFrom(base) }} ·
                <v-btn variant="text" size="small" class="restore" :aria-label="t.restoreServingsLabel(base)" @click="setServings(base)">{{ t.restoreServings }}</v-btn>
              </span>
            </template>
            <ul v-if="ingredients.length" class="list-plain">
              <li v-for="(i, n) in ingredients" :key="n" class="ingredient">
                <span class="ingredient__qty mp-num" :aria-hidden="i.amount ? undefined : 'true'">{{ i.amount }}</span>{{ ' ' }}<span class="ingredient__name">{{ i.name }}<template v-if="i.note">{{ ' ' }}<span class="ingredient__note text-medium-emphasis">{{ i.note }}</span></template></span>
              </li>
            </ul>
          </NoteCard>
        </aside>

        <div class="recipe-view__main">
          <template v-if="recipe.steps.length">
            <h2 class="text-h5 mb-2">{{ t.steps }}</h2>
            <ol class="steps" role="list">
              <li v-for="(s, n) in recipe.steps" :key="n">{{ s }}</li>
            </ol>
          </template>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.recipe-view { max-width: var(--mp-content-max); margin: 0 auto; }
.title { overflow-wrap: anywhere; }
.source { color: rgb(var(--v-theme-primary)); }
.recipe-head { display: grid; gap: 8px; }
.recipe-acts { display: flex; align-items: center; flex-wrap: wrap; gap: 8px 16px; }
.recipe-view--columns .recipe-head { grid-template-columns: minmax(0, 1fr) auto; align-items: end; }
.recipe-view--columns .recipe-head .tags { grid-row: 2; }
.recipe-view--columns .recipe-head .recipe-acts { grid-column: 2; grid-row: 1 / span 2; }
.recipe-view--columns .recipe-head .nutrition { grid-column: 1; }
.recipe-body { display: grid; gap: 32px; margin-top: 24px; }
.recipe-view--columns .recipe-body { grid-template-columns: minmax(0, 450px) minmax(0, 1fr); align-items: start; gap: 48px; }
.recipe-view--columns .recipe-view__aside {
  /* Stick below the fixed app bar (v-main exposes its height) and scroll inside when the list is taller than the viewport. */
  --aside-top: calc(var(--v-layout-top, 64px) + 16px);
  position: sticky;
  top: var(--aside-top);
  max-height: calc(100vh - var(--aside-top) - 16px);
  overflow-y: auto;
  /* Room for the note card shadow, which the scroll container would otherwise clip. */
  padding: 4px 8px 12px 4px;
  margin: -4px -8px -12px -4px;
}
.stepper { display: flex; align-items: center; gap: 8px; }
.step-btn { min-width: 44px; min-height: 44px; }
.servings { min-width: 2ch; text-align: center; }
.list-plain { list-style: none; padding: 0; margin: 0; }
.ingredient { display: flex; gap: 0 12px; padding: 8px 0; border-bottom: 1px solid var(--mp-line); }
.ingredient:last-child { border-bottom: 0; }
.ingredient__qty { flex: 0 0 5.5em; text-align: right; font-weight: 500; white-space: nowrap; }
.ingredient__name { flex: 1 1 auto; min-width: 0; font-size: 1.125rem; line-height: 1.3; }
.ingredient__note { display: block; font-size: 0.875rem; }
.steps { list-style: none; padding: 0; margin: 0; counter-reset: step; border-top: 1px solid var(--mp-rule); }
.steps > li { counter-increment: step; display: grid; grid-template-columns: 2.5rem minmax(0, 1fr); gap: 12px; padding: 16px 0; border-bottom: 1px solid var(--mp-line); font-size: 1.125rem; line-height: 1.5; }
.steps > li::before { content: counter(step); font-size: 1.75rem; line-height: 1.1; font-variant-numeric: tabular-nums; font-family: var(--v-font-display); }
</style>
