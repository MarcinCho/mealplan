<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import type { PantryItem } from '../api/types'
import { pantryErrorText, t } from '../messages'
import { formatStock } from '../pantry'
import { usePantry } from '../stores/pantry'
import PantryAddDialog from './PantryAddDialog.vue'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()

const { mdAndUp } = useDisplay()
const pantry = usePantry()
const queue = ref<string[]>([]) // ids fixed when the review opens, so edits do not reshuffle it
const index = ref(0)
const quantity = ref<number | string | null>(null)
const error = ref('')
const busy = ref(false)
const adding = ref(false)

const total = computed(() => queue.value.length)
const current = computed<PantryItem | null>(() => pantry.items.find((i) => i.id === queue.value[index.value]) ?? null)
const finished = computed(() => index.value >= total.value)
const changed = computed(() => {
  const c = current.value
  if (!c || c.staple || quantity.value === '' || quantity.value === null) return false
  return Number(quantity.value) !== c.quantity
})
const valid = computed(() => {
  if (current.value?.staple) return true
  const q = quantity.value
  return q !== '' && q !== null && Number.isFinite(Number(q)) && Number(q) >= 0 && Number(q) <= 1000000
})

function syncQuantity() {
  quantity.value = current.value?.quantity ?? null
}

watch(
  () => props.modelValue,
  (open) => {
    if (!open) return
    queue.value = [...pantry.items].sort((a, b) => a.name.localeCompare(b.name, 'pl')).map((i) => i.id)
    index.value = 0
    error.value = ''
    syncQuantity()
  },
)

function next() {
  index.value++
  error.value = ''
  syncQuantity()
}

async function guarded(fn: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await fn()
  } catch (e) {
    error.value = pantryErrorText(e)
  } finally {
    busy.value = false
  }
}

const keep = () =>
  guarded(async () => {
    const c = current.value
    if (c && changed.value && valid.value) {
      pantry.cancelSaves(c.id) // an explicit review answer wins over pending stepper saves
      await pantry.update(c.id, { quantity: Number(quantity.value), unit: c.unit, expiresOn: c.expiresOn, staple: c.staple })
    }
    next()
  })

const drop = () =>
  guarded(async () => {
    const c = current.value
    if (c) await pantry.remove(c.id)
    // The row is gone from the queue's point of view: skip over it without advancing.
    queue.value.splice(index.value, 1)
    error.value = ''
    syncQuantity()
  })

const finish = () =>
  guarded(async () => {
    await pantry.markReviewed()
    emit('update:modelValue', false)
  })
</script>

<template>
  <v-dialog :model-value="modelValue" :fullscreen="!mdAndUp" max-width="520" scrollable :aria-label="t.pantryReviewTitle" @update:model-value="emit('update:modelValue', $event)">
    <v-card class="review">
      <v-toolbar flat density="comfortable">
        <v-btn icon="mdi-close" :title="t.close" @click="emit('update:modelValue', false)" />
        <v-toolbar-title>{{ t.pantryReviewTitle }}</v-toolbar-title>
        <span v-if="!finished" class="text-body-2 mr-4 progress">{{ index + 1 }} / {{ total }}</span>
      </v-toolbar>
      <v-progress-linear :model-value="total ? (Math.min(index, total) / total) * 100 : 0" color="primary" />
      <v-card-text class="pt-6">
        <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
        <template v-if="!finished && current">
          <div class="text-h5 mb-1 item-name">{{ current.name }}</div>
          <div class="text-body-1 text-medium-emphasis mb-1">{{ formatStock(current) }}</div>
          <div v-if="current.expiresOn" class="text-body-2 text-medium-emphasis mb-4">{{ t.pantryExpires }} {{ current.expiresOn }}</div>
          <v-text-field
            v-if="!current.staple"
            v-model.number="quantity"
            :label="`${t.pantryReviewQuantity} (${current.unit})`"
            type="number"
            min="0"
            inputmode="decimal"
            class="mt-4"
            @keyup.enter="keep"
          />
          <div class="d-flex flex-column ga-2 mt-2">
            <v-btn color="primary" size="large" block :disabled="!valid" :loading="busy" @click="keep">
              {{ changed ? t.pantryReviewSave : t.pantryReviewSame }}
            </v-btn>
            <v-btn color="error" variant="tonal" size="large" block :disabled="busy" @click="drop">{{ t.pantryReviewNone }}</v-btn>
          </div>
        </template>
        <div v-else class="text-h6 text-center py-6">{{ total ? t.pantryReviewAllSeen : t.pantryReviewEmpty }}</div>
      </v-card-text>
      <v-card-actions class="flex-wrap ga-2 pa-4">
        <v-btn variant="text" prepend-icon="mdi-plus" @click="adding = true">{{ t.pantryReviewAddMissing }}</v-btn>
        <v-spacer />
        <v-btn :color="finished ? 'primary' : undefined" :variant="finished ? 'flat' : 'text'" :loading="busy" @click="finish">{{ t.pantryReviewFinish }}</v-btn>
      </v-card-actions>
    </v-card>
    <PantryAddDialog v-model="adding" />
  </v-dialog>
</template>

<style scoped>
.item-name { overflow-wrap: anywhere; }
.progress { white-space: nowrap; }
</style>
