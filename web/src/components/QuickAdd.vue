<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Recipe, Slot } from '../api/types'
import { t } from '../messages'
import type { EntryInput } from '../stores/plan'
import type { Member } from '../stores/members'
import { SLOTS } from '../weeks'
import { suggest } from '../today'

const props = defineProps<{ recipes: Recipe[]; members: Member[]; dates: string[]; date: string }>()
const emit = defineEmits<{ (e: 'add', input: EntryInput): void }>()

const text = ref('')
const picked = ref<Recipe | null>(null)
const day = ref(props.date)
const slot = ref<Slot>('lunch')
const cook = ref<string | null>(null)

watch(() => props.date, (d) => (day.value = d))

const hits = computed(() => (picked.value ? [] : suggest(props.recipes, text.value)))
const open = computed(() => picked.value !== null || text.value.trim() !== '')

function pick(r: Recipe) {
  picked.value = r
  text.value = r.title
}

function onInput() {
  // typing again after a pick starts a new search
  if (picked.value && text.value !== picked.value.title) picked.value = null
}

function reset() {
  text.value = ''
  picked.value = null
  cook.value = null
}

function submit() {
  const title = text.value.trim()
  if (!title) return
  emit('add', {
    date: day.value,
    slot: slot.value,
    recipeId: picked.value?.id ?? null,
    freeText: picked.value ? '' : title,
    cookUserId: cook.value,
  })
  reset()
}

const dayLabel = (d: string) => t.dayNames[props.dates.indexOf(d)]
</script>

<template>
  <div class="quick">
    <div class="quick-field">
      <v-text-field
        v-model="text"
        :placeholder="t.quickPlaceholder"
        variant="plain"
        density="comfortable"
        hide-details
        class="px-3"
        maxlength="300"
        @update:model-value="onInput"
        @keydown.enter="picked ? submit() : hits[0] ? pick(hits[0]) : undefined"
      />
      <v-btn icon="mdi-plus" color="primary" :title="t.addMeal" :disabled="!open" class="mr-1" @click="submit" />
    </div>
    <v-list v-if="hits.length" density="compact" class="mt-1" bg-color="transparent">
      <v-list-item v-for="r in hits" :key="r.id" :title="r.title" prepend-icon="mdi-book-open-variant" @click="pick(r)" />
    </v-list>
    <div v-if="open" class="pa-3 pt-2">
      <div v-if="!picked && !hits.length" class="text-caption text-medium-emphasis mb-2">{{ t.quickFreeText }}</div>
      <v-chip-group v-model="day" mandatory selected-class="bg-primary" class="mb-1">
        <v-chip v-for="d in dates" :key="d" :value="d" size="small" filter>{{ dayLabel(d) }}</v-chip>
      </v-chip-group>
      <v-chip-group v-model="slot" mandatory selected-class="bg-primary" class="mb-1">
        <v-chip v-for="s in SLOTS" :key="s.value" :value="s.value" size="small">{{ s.title }}</v-chip>
      </v-chip-group>
      <v-chip-group v-model="cook" selected-class="bg-primary">
        <v-chip v-for="m in members" :key="m.id" :value="m.id" size="small" prepend-icon="mdi-account">{{ m.displayName }}</v-chip>
      </v-chip-group>
    </div>
  </div>
</template>

<style scoped>
.quick {
  background: rgb(var(--v-theme-surface));
  border: 1px solid rgba(var(--v-theme-on-surface), 0.12);
  border-radius: 28px;
  box-shadow: 0 8px 28px rgba(0, 0, 0, 0.12);
}
.quick-field {
  display: flex;
  align-items: center;
  padding: 4px;
}
.quick-field :deep(.v-field) {
  --v-field-padding-start: 12px;
}
</style>
