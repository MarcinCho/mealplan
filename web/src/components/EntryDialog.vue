<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { VBottomSheet, VDialog } from 'vuetify/components'
import type { Macros, PlanEntry, Recipe, Slot } from '../api/types'
import { formatDay, SLOTS } from '../weeks'
import { t } from '../messages'
import { useMealPrepStore } from '../stores/mealprep'
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
const prep = useMealPrepStore()

type Kind = 'recipe' | 'text' | 'prep' | 'out'
const kind = ref<Kind>('recipe')
const date = ref('')
const slot = ref<Slot>('dinner')
const recipeId = ref<string | null>(null)
const freeText = ref('')
const servings = ref<number | null>(null)
const cookUserId = ref<string | null>(null)
// 'd:<dishId>' or 'f:<freezerItemId>'
const source = ref<string | null>(null)
const outText = ref('')
const out = ref<Record<keyof Macros, string | number | null>>({ kcal: null, protein: null, carbs: null, fat: null })

// An existing prep or out entry keeps its kind; only its details change.
const fixed = computed(() => props.entry?.kind === 'prep' || props.entry?.kind === 'out')

watch(
  () => [props.modelValue, props.entry, props.initial] as const,
  () => {
    if (!props.modelValue) return
    const e = props.entry
    date.value = e?.date ?? props.initial.date
    slot.value = e?.slot ?? props.initial.slot
    kind.value = e?.kind === 'prep' || e?.kind === 'out' ? e.kind : e && !e.recipeId ? 'text' : 'recipe'
    recipeId.value = e?.recipeId ?? null
    freeText.value = e?.freeText ?? ''
    servings.value = e?.servings ?? null
    cookUserId.value = e?.cookUserId ?? null
    source.value = e?.prepDishId ? `d:${e.prepDishId}` : e?.freezerItemId ? `f:${e.freezerItemId}` : null
    outText.value = e?.kind === 'out' ? e.freeText : ''
    out.value = {
      kcal: e?.outValues?.kcal ?? null,
      protein: e?.outValues?.protein ?? null,
      carbs: e?.outValues?.carbs ?? null,
      fat: e?.outValues?.fat ?? null,
    }
    // The meal prep list needs the store; a failure only leaves the list empty.
    if (!prep.loaded) prep.load().catch(() => {})
  },
  { immediate: true },
)

interface Source { value: string; title: string; remaining: number }
// While editing, the entry's own portions count as available again.
const own = computed(() => (props.entry?.kind === 'prep' ? props.entry.servings : 0))
const sources = computed<Source[]>(() => {
  const list: Source[] = []
  for (const s of prep.sessions)
    for (const d of s.dishes) {
      const value = `d:${d.id}`
      const remaining = d.remaining + (value === source.value ? own.value : 0)
      if (remaining > 0) list.push({ value, title: `${d.recipeTitle} · ${t.entryPrepLeft} ${remaining}`, remaining })
    }
  for (const f of prep.freezer) {
    const value = `f:${f.id}`
    const remaining = f.remaining + (value === source.value ? own.value : 0)
    if (remaining > 0) list.push({ value, title: `❄ ${f.title} · ${t.entryPrepLeft} ${remaining}`, remaining })
  }
  return list
})
const maxServings = computed(() => sources.value.find((x) => x.value === source.value)?.remaining ?? 0)

// Choosing a new source suggests the household's usual portions, capped by what is left.
function pick(v: string | null) {
  source.value = v
  const rest = sources.value.find((x) => x.value === v)?.remaining ?? 0
  if (rest > 0) servings.value = Math.max(1, Math.min(prep.portionsPerMeal, rest))
}

const dayItems = computed(() => props.dates.map((d) => ({ title: formatDay(d), value: d })))
const prepValid = computed(() => {
  const n = servings.value
  return !!source.value && !!n && Number.isInteger(n) && n >= 1 && n <= maxServings.value
})
const valid = computed(() => {
  if (kind.value === 'recipe') return !!recipeId.value
  if (kind.value === 'text') return freeText.value.trim() !== ''
  if (kind.value === 'prep') return prepValid.value
  return true
})

const num = (v: string | number | null) => (v === null || v === '' ? null : Number(v))

function save() {
  const base = { date: date.value, slot: slot.value, done: props.entry?.done ?? false }
  if (kind.value === 'prep') {
    emit('save', {
      ...base,
      kind: 'prep',
      prepDishId: source.value?.startsWith('d:') ? source.value.slice(2) : null,
      freezerItemId: source.value?.startsWith('f:') ? source.value.slice(2) : null,
      servings: servings.value || 0,
    })
  } else if (kind.value === 'out') {
    emit('save', {
      ...base,
      kind: 'out',
      freeText: outText.value,
      outValues: { kcal: num(out.value.kcal), protein: num(out.value.protein), carbs: num(out.value.carbs), fat: num(out.value.fat) },
    })
  } else {
    emit('save', {
      ...base,
      kind: 'meal',
      recipeId: kind.value === 'recipe' ? recipeId.value : null,
      freeText: kind.value === 'text' ? freeText.value : '',
      servings: servings.value || 0,
      cookUserId: cookUserId.value,
    })
  }
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
        <v-chip-group v-model="kind" mandatory selected-class="text-primary" color="primary" class="mb-4" column :disabled="fixed" :aria-label="t.entryType">
          <v-chip value="recipe" filter variant="outlined">{{ t.entryRecipe }}</v-chip>
          <v-chip value="text" filter variant="outlined">{{ t.entryFreeText }}</v-chip>
          <v-chip value="prep" filter variant="outlined">{{ t.entryPrep }}</v-chip>
          <v-chip value="out" filter variant="outlined">{{ t.entryOut }}</v-chip>
        </v-chip-group>
        <v-select v-if="kind === 'recipe'" v-model="recipeId" :items="recipes" item-title="title" item-value="id" :label="t.entryRecipe" />
        <v-text-field v-else-if="kind === 'text'" v-model="freeText" :label="t.entryFreeText" maxlength="300" />
        <template v-else-if="kind === 'prep'">
          <v-select
            :model-value="source"
            :items="sources"
            item-title="title"
            item-value="value"
            :label="t.entryPrepSource"
            :disabled="!!entry"
            :no-data-text="t.entryPrepNone"
            @update:model-value="pick"
          />
          <v-text-field v-model.number="servings" :label="t.entryServings" type="number" min="1" :max="maxServings || undefined" />
        </template>
        <template v-else>
          <v-text-field v-model="outText" :label="t.entryOutDescription" maxlength="300" />
          <div class="text-body-2 text-medium-emphasis mb-2">{{ t.entryOutValues }}</div>
          <v-row dense>
            <v-col cols="6" md="3"><v-text-field v-model="out.kcal" :label="t.macroKcal" suffix="kcal" type="number" min="0" /></v-col>
            <v-col cols="6" md="3"><v-text-field v-model="out.protein" :label="t.macroProtein" suffix="g" type="number" min="0" /></v-col>
            <v-col cols="6" md="3"><v-text-field v-model="out.carbs" :label="t.macroCarbs" suffix="g" type="number" min="0" /></v-col>
            <v-col cols="6" md="3"><v-text-field v-model="out.fat" :label="t.macroFat" suffix="g" type="number" min="0" /></v-col>
          </v-row>
        </template>
        <template v-if="kind === 'recipe' || kind === 'text'">
          <v-text-field v-model.number="servings" :label="t.entryServings" type="number" min="1" max="100" />
          <v-select v-model="cookUserId" :items="members" item-title="displayName" item-value="id" :label="t.entryCook" clearable :placeholder="t.entryNoCook" />
        </template>
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
