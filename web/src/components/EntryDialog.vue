<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { VBottomSheet, VDialog } from 'vuetify/components'
import type { PlanEntry, Recipe, Slot } from '../api/types'
import { formatDay, SLOTS } from '../weeks'
import { t } from '../messages'
import type { EntryInput } from '../stores/plan'

interface Member { id: string; displayName: string }

const props = defineProps<{
  modelValue: boolean
  dates: string[]
  recipes: Recipe[]
  members: Member[]
  entry: PlanEntry | null
  initial: { date: string; slot: Slot }
}>()
const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'save', input: EntryInput): void
  (e: 'remove'): void
}>()

const { mdAndUp } = useDisplay()

const kind = ref<'recipe' | 'text'>('recipe')
const date = ref('')
const slot = ref<Slot>('dinner')
const recipeId = ref<string | null>(null)
const freeText = ref('')
const servings = ref<number | null>(null)
const cookUserId = ref<string | null>(null)

watch(
  () => [props.modelValue, props.entry, props.initial] as const,
  () => {
    if (!props.modelValue) return
    const e = props.entry
    date.value = e?.date ?? props.initial.date
    slot.value = e?.slot ?? props.initial.slot
    kind.value = e && !e.recipeId ? 'text' : 'recipe'
    recipeId.value = e?.recipeId ?? null
    freeText.value = e?.freeText ?? ''
    servings.value = e?.servings ?? null
    cookUserId.value = e?.cookUserId ?? null
  },
  { immediate: true },
)

const dayItems = computed(() => props.dates.map((d) => ({ title: formatDay(d), value: d })))
const valid = computed(() => (kind.value === 'recipe' ? !!recipeId.value : freeText.value.trim() !== ''))

function save() {
  emit('save', {
    date: date.value,
    slot: slot.value,
    recipeId: kind.value === 'recipe' ? recipeId.value : null,
    freeText: kind.value === 'text' ? freeText.value : '',
    servings: servings.value || 0,
    cookUserId: cookUserId.value,
    done: props.entry?.done ?? false,
  })
}
</script>

<template>
  <component
    :is="mdAndUp ? VDialog : VBottomSheet"
    :model-value="modelValue"
    max-width="480"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <v-card>
      <v-card-title>{{ entry ? t.save : t.addEntry }}</v-card-title>
      <v-card-text>
        <v-select v-model="date" :items="dayItems" :label="t.entryDay" />
        <v-select v-model="slot" :items="SLOTS" item-title="title" item-value="value" :label="t.entrySlot" />
        <v-btn-toggle v-model="kind" mandatory density="comfortable" class="mb-4" color="primary">
          <v-btn value="recipe">{{ t.entryRecipe }}</v-btn>
          <v-btn value="text">{{ t.entryFreeText }}</v-btn>
        </v-btn-toggle>
        <v-select v-if="kind === 'recipe'" v-model="recipeId" :items="recipes" item-title="title" item-value="id" :label="t.entryRecipe" />
        <v-text-field v-else v-model="freeText" :label="t.entryFreeText" maxlength="300" />
        <v-text-field v-model.number="servings" :label="t.entryServings" type="number" min="1" max="100" />
        <v-select v-model="cookUserId" :items="members" item-title="displayName" item-value="id" :label="t.entryCook" clearable :placeholder="t.entryNoCook" />
      </v-card-text>
      <v-card-actions>
        <v-btn v-if="entry" color="error" variant="text" @click="emit('remove')">{{ t.delete }}</v-btn>
        <v-spacer />
        <v-btn variant="text" @click="emit('update:modelValue', false)">{{ t.cancel }}</v-btn>
        <v-btn color="primary" :disabled="!valid" @click="save">{{ t.save }}</v-btn>
      </v-card-actions>
    </v-card>
  </component>
</template>
