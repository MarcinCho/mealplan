<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { VBottomSheet, VDialog } from 'vuetify/components'
import type { Slot } from '../api/types'
import { errorText, t } from '../messages'
import { isoDate, mondayOf, SLOTS } from '../weeks'
import { usePlan } from '../stores/plan'
import { useMealPrepStore } from '../stores/mealprep'

export interface AllocateSource {
  dishId?: string
  freezerItemId?: string
  title: string
  remaining: number
}

const props = defineProps<{ modelValue: boolean; source: AllocateSource }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()

const { mdAndUp } = useDisplay()
const plan = usePlan()
const prep = useMealPrepStore()

const date = ref(isoDate(new Date()))
const slot = ref<Slot>('dinner')
const servings = ref<number | null>(1)
const error = ref('')
const busy = ref(false)

watch(
  () => props.modelValue,
  (open) => {
    if (!open) return
    date.value = isoDate(new Date())
    slot.value = 'dinner'
    servings.value = Math.max(1, Math.min(prep.portionsPerMeal, props.source.remaining))
    error.value = ''
  },
  { immediate: true },
)

const valid = computed(() => {
  const n = servings.value
  return /^\d{4}-\d{2}-\d{2}$/.test(date.value) && !!n && Number.isInteger(n) && n >= 1 && n <= props.source.remaining
})

async function save() {
  if (!valid.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    const [y, m, d] = date.value.split('-').map(Number)
    await plan.add(
      {
        date: date.value,
        slot: slot.value,
        kind: 'prep',
        prepDishId: props.source.dishId ?? null,
        freezerItemId: props.source.freezerItemId ?? null,
        servings: servings.value ?? 1,
      },
      mondayOf(new Date(y, m - 1, d)),
    )
    await prep.reload()
    emit('update:modelValue', false)
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <component :is="mdAndUp ? VDialog : VBottomSheet" :model-value="modelValue" max-width="480" @update:model-value="emit('update:modelValue', $event)">
    <v-card>
      <v-card-title>{{ t.allocateTitle }}</v-card-title>
      <v-card-text>
        <div class="font-weight-bold mb-1">{{ source.title }}</div>
        <div class="text-body-2 text-medium-emphasis mb-4">{{ t.allocateAvailable }}: {{ source.remaining }}</div>
        <v-text-field v-model="date" type="date" :label="t.allocateDate" />
        <v-select v-model="slot" :items="SLOTS" item-title="title" item-value="value" :label="t.entrySlot" />
        <v-text-field v-model.number="servings" type="number" min="1" :max="source.remaining" :label="t.allocateServings" />
        <v-alert v-if="error" type="error" variant="tonal" density="compact">{{ error }}</v-alert>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="emit('update:modelValue', false)">{{ t.cancel }}</v-btn>
        <v-btn color="primary" :disabled="!valid" :loading="busy" @click="save">{{ t.prepAllocate }}</v-btn>
      </v-card-actions>
    </v-card>
  </component>
</template>
