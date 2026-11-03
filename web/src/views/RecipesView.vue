<script setup lang="ts">
import { onMounted, ref } from 'vue'
import EmptyState from '../components/EmptyState.vue'
import { useRecipes } from '../stores/recipes'
import { errorText, t } from '../messages'

const recipes = useRecipes()
const error = ref('')

onMounted(async () => {
  try {
    await recipes.load()
  } catch (e) {
    error.value = errorText(e)
  }
})
</script>

<template>
  <div class="d-flex align-center mb-4">
    <h1 class="text-h5">{{ t.recipes }}</h1>
    <v-spacer />
    <v-btn variant="tonal" :to="{ name: 'recipe-import' }" prepend-icon="mdi-download" class="mr-2">{{ t.importRecipe }}</v-btn>
    <v-btn color="primary" :to="{ name: 'recipe-new' }" prepend-icon="mdi-plus">{{ t.newRecipe }}</v-btn>
  </div>
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
  <EmptyState v-if="recipes.loaded && !recipes.items.length" :title="t.noRecipes" :hint="t.noRecipesHint" />
  <v-list v-else lines="two" border rounded>
    <v-list-item
      v-for="r in recipes.items"
      :key="r.id"
      :to="{ name: 'recipe-edit', params: { id: r.id } }"
      :title="r.title"
      :subtitle="`${r.baseServings} ${t.servingsShort}`"
    >
      <template #append>
        <v-chip v-for="tag in r.tags" :key="tag" size="small" class="ml-1">{{ tag }}</v-chip>
      </template>
    </v-list-item>
  </v-list>
</template>
