<script setup lang="ts">
import { computed } from 'vue'
import type { DaySummary, Macros } from '../api/types'
import { t } from '../messages'
import { dayWarnings, formatNumber, formatRemaining, goalPercent } from '../nutrition'

const props = defineProps<{ day: DaySummary }>()
const emit = defineEmits<{ (e: 'add'): void; (e: 'remove', id: string): void }>()

const ROWS: { key: keyof Macros; label: string; unit: 'kcal' | 'g' }[] = [
  { key: 'kcal', label: t.macroKcal, unit: 'kcal' },
  { key: 'protein', label: t.macroProtein, unit: 'g' },
  { key: 'carbs', label: t.macroCarbs, unit: 'g' },
  { key: 'fat', label: t.macroFat, unit: 'g' },
]

const rows = computed(() =>
  ROWS.map((r) => {
    const total = props.day.total[r.key]
    const goal = props.day.goal[r.key]
    return {
      ...r,
      text: formatNumber(total, r.unit),
      goalText: goal === null || goal === undefined ? '' : ` / ${formatNumber(goal, r.unit)}`,
      percent: goalPercent(total, goal),
      partial: props.day.incomplete[r.key],
    }
  }),
)
const warnings = computed(() => dayWarnings(props.day))
const remaining = computed(() => formatRemaining(props.day))
const itemValue = (v: Macros) => (v.kcal === null ? '—' : `${formatNumber(v.kcal, 'kcal')} kcal`)
</script>

<template>
  <v-card class="pa-4 mb-4 day-summary">
    <div class="d-flex align-center mb-3">
      <h3 class="text-subtitle-1 font-weight-bold flex-grow-1">{{ t.daySummary }}</h3>
      <v-btn size="small" variant="tonal" color="primary" prepend-icon="mdi-plus" @click="emit('add')">{{ t.logAdd }}</v-btn>
    </div>

    <div v-for="r in rows" :key="r.key" class="mb-2">
      <div class="d-flex justify-space-between text-body-2">
        <span>{{ r.label }}</span>
        <span>
          <b>{{ r.text }}</b>{{ r.goalText }} {{ r.unit === 'kcal' ? 'kcal' : 'g' }}
        </span>
      </div>
      <v-progress-linear
        v-if="r.percent !== null"
        :model-value="r.percent"
        :aria-label="r.label"
        height="6"
        rounded
        :color="r.partial ? 'warning' : 'accent'"
      />
    </div>

    <div class="text-body-2 mt-3">{{ remaining }}</div>
    <div v-if="warnings.length" class="d-flex flex-wrap ga-2 mt-2">
      <v-chip v-for="w in warnings" :key="w" size="small" color="warning" variant="tonal" prepend-icon="mdi-alert-outline">{{ w }}</v-chip>
    </div>

    <div class="section-label mt-4">{{ t.logListTitle }}</div>
    <div v-if="!day.log.length" class="text-body-2 text-medium-emphasis">{{ t.logEmpty }}</div>
    <div v-for="l in day.log" :key="l.id" class="d-flex align-center ga-2 log-row">
      <div class="flex-grow-1" style="min-width: 0">
        <div class="log-name">{{ l.name }}</div>
        <div class="text-body-2 text-medium-emphasis">{{ formatNumber(l.quantity, 'g') }} {{ l.unit }} · {{ itemValue(l.values) }}</div>
      </div>
      <v-btn icon="mdi-delete-outline" variant="text" size="small" :title="t.logRemove" @click="emit('remove', l.id)" />
    </div>
  </v-card>
</template>

<style scoped>
.section-label { font-size: 12px; font-weight: 700; letter-spacing: 0.8px; text-transform: uppercase; color: rgba(var(--v-theme-on-surface), var(--v-medium-emphasis-opacity)); margin-bottom: 4px; }
.log-name { overflow-wrap: anywhere; }
</style>
