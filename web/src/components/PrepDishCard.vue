<script setup lang="ts">
import { computed } from 'vue'
import type { PrepDish } from '../api/types'
import { allocationLabel } from '../mealprep'
import { t } from '../messages'

const props = defineProps<{ dish: PrepDish }>()
const emit = defineEmits<{
  (e: 'allocate'): void
  (e: 'freeze'): void
  (e: 'writeoff'): void
  (e: 'edit'): void
  (e: 'remove'): void
}>()

const total = computed(() => Math.max(props.dish.portions, 1))
const pct = (n: number) => `${Math.max(0, Math.min(100, (n / total.value) * 100))}%`
</script>

<template>
  <v-card variant="tonal" color="surface-variant" class="pa-3 mb-2">
    <div class="d-flex align-center ga-2">
      <div class="font-weight-bold flex-grow-1">{{ dish.recipeTitle }}</div>
      <span class="text-body-2">{{ dish.portions }} {{ t.prepPortions.toLowerCase() }}</span>
      <v-btn icon="mdi-pencil-outline" size="small" variant="text" :title="t.prepEditDish" @click="emit('edit')" />
      <v-btn icon="mdi-delete-outline" size="small" variant="text" :title="t.prepDishDelete" @click="emit('remove')" />
    </div>
    <div class="bar my-2" role="img" :aria-label="`${dish.allocated} ${t.prepInPlan}, ${dish.frozen} ${t.prepInFreezer}, ${dish.remaining} ${t.prepLeft}`">
      <span class="seg plan" :style="{ width: pct(dish.allocated) }" />
      <span class="seg freezer" :style="{ width: pct(dish.frozen) }" />
      <span class="seg left" :style="{ width: pct(dish.remaining) }" />
    </div>
    <div class="d-flex flex-wrap ga-3 text-body-2 mb-1">
      <span><i class="dot plan" /> {{ t.prepInPlan }} {{ dish.allocated }}</span>
      <span><i class="dot freezer" /> {{ t.prepInFreezer }} {{ dish.frozen }}</span>
      <span><i class="dot left" /> {{ t.prepLeft }} {{ dish.remaining }}</span>
    </div>
    <div v-if="dish.allocations.length" class="d-flex flex-wrap ga-1 mb-2">
      <v-chip v-for="a in dish.allocations" :key="a.entryId" size="small" label>{{ allocationLabel(a) }}</v-chip>
    </div>
    <div class="d-flex flex-wrap ga-2">
      <v-btn size="small" color="primary" variant="tonal" :disabled="dish.remaining < 1" @click="emit('allocate')">{{ t.prepAllocate }}</v-btn>
      <v-btn size="small" variant="tonal" :disabled="dish.remaining < 1" @click="emit('freeze')">{{ t.prepFreeze }}</v-btn>
      <v-btn size="small" variant="text" @click="emit('writeoff')">{{ t.prepWriteOff }}</v-btn>
    </div>
  </v-card>
</template>

<style scoped>
.bar { display: flex; height: 10px; border-radius: 5px; overflow: hidden; background: rgba(var(--v-theme-on-surface), 0.12); }
.seg.plan, .dot.plan { background: rgb(var(--v-theme-primary)); }
.seg.freezer, .dot.freezer { background: rgb(var(--v-theme-secondary)); }
.seg.left, .dot.left { background: rgba(var(--v-theme-on-surface), 0.3); }
.dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; }
</style>
