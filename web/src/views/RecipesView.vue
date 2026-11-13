<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import EmptyState from '../components/EmptyState.vue'
import { portions } from '../polish'
import { useRecipes } from '../stores/recipes'
import { errorText, t } from '../messages'

const recipes = useRecipes()
const error = ref('')
const query = ref('')

// Matches the title or a tag, ignoring case and Polish diacritics ("zupa" finds "Zupa ogórkowa", "sniad" finds "śniadanie").
const fold = (s: string) => s.toLocaleLowerCase('pl-PL').normalize('NFD').replace(/\p{Diacritic}/gu, '').replace(/ł/g, 'l')
const shown = computed(() => {
  const q = fold(query.value.trim())
  if (!q) return recipes.items
  return recipes.items.filter((r) => fold(r.title).includes(q) || r.tags.some((tag) => fold(tag).includes(q)))
})

onMounted(async () => {
  try {
    await recipes.load()
  } catch (e) {
    error.value = errorText(e)
  }
})
</script>

<template>
  <div class="d-flex align-center flex-wrap ga-2 mb-4">
    <h1 class="text-h5 mr-auto">{{ t.recipes }}</h1>
    <div class="d-flex flex-wrap ga-2">
      <v-btn variant="tonal" :to="{ name: 'recipe-import' }" prepend-icon="mdi-download">{{ t.importRecipe }}</v-btn>
      <v-btn color="primary" :to="{ name: 'recipe-new' }" prepend-icon="mdi-plus">{{ t.newRecipe }}</v-btn>
    </div>
  </div>
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
  <EmptyState v-if="recipes.loaded && !recipes.items.length" :title="t.noRecipes" :hint="t.noRecipesHint" />
  <template v-else>
    <v-text-field
      v-if="recipes.items.length > 3"
      v-model="query"
      :label="t.searchRecipes"
      prepend-inner-icon="mdi-magnify"
      clearable
      hide-details
      class="mb-3"
    />
    <div v-if="query && !shown.length" class="text-body-2 text-medium-emphasis pa-4">{{ t.noRecipesFound }}</div>
    <v-card v-else>
      <v-list lines="two" role="none">
        <v-list-item v-for="r in shown" :key="r.id" :to="{ name: 'recipe-edit', params: { id: r.id } }" :title="r.title">
          <v-list-item-subtitle class="d-flex flex-wrap align-center ga-1 mt-1">
            <span class="mr-1">{{ portions(r.baseServings) }}</span>
            <v-chip v-for="tag in r.tags" :key="tag" size="x-small" variant="tonal">{{ tag }}</v-chip>
          </v-list-item-subtitle>
        </v-list-item>
      </v-list>
    </v-card>
  </template>
</template>
