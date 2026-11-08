<script setup lang="ts">
import { computed } from 'vue'
import type { Macros, MealNutrition, Source } from '../api/types'
import { t } from '../messages'
import { formatNumber } from '../nutrition'

const props = defineProps<{ modelValue: boolean; meal: MealNutrition | null }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()

const ROWS: { key: keyof Macros; label: string; unit: 'kcal' | 'g' }[] = [
  { key: 'kcal', label: t.macroKcal, unit: 'kcal' },
  { key: 'protein', label: t.macroProtein, unit: 'g' },
  { key: 'carbs', label: t.macroCarbs, unit: 'g' },
  { key: 'fat', label: t.macroFat, unit: 'g' },
]
const SOURCE: Record<Source, string> = { recipe: t.nutritionFromRecipe, computed: t.nutritionComputed, missing: t.nutritionMissing }

const rows = computed(() =>
  props.meal
    ? ROWS.map((r) => ({
        ...r,
        value: formatNumber(props.meal!.values[r.key], r.unit) + (props.meal!.values[r.key] === null ? '' : r.unit === 'kcal' ? ' kcal' : ' g'),
        source: SOURCE[props.meal!.sources[r.key]],
      }))
    : [],
)
</script>

<template>
  <v-dialog :model-value="modelValue" max-width="420" @update:model-value="emit('update:modelValue', $event)">
    <v-card v-if="meal">
      <v-card-title class="text-wrap">{{ t.mealNutritionTitle }}</v-card-title>
      <v-card-subtitle class="text-wrap">{{ meal.title }}</v-card-subtitle>
      <v-card-text>
        <div v-for="r in rows" :key="r.key" class="d-flex align-baseline ga-3 py-1">
          <span class="flex-grow-1">{{ r.label }}</span>
          <b>{{ r.value }}</b>
          <span class="text-body-2 text-medium-emphasis source">{{ r.source }}</span>
        </div>
        <v-alert v-if="meal.approximate" type="warning" variant="tonal" density="compact" class="mt-3">{{ t.approximate }}</v-alert>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="emit('update:modelValue', false)">{{ t.close }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.source { min-width: 72px; text-align: end; }
</style>
