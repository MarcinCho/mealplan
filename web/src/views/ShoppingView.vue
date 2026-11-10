<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import EmptyState from '../components/EmptyState.vue'
import { errorText, movedText, t, undonePreviousText } from '../messages'
import { useShopping } from '../stores/shopping'
import { shoppingLine } from '../pantry'
import { usePantry } from '../stores/pantry'
import { formatDay, mondayOf, shiftWeek, weekDates } from '../weeks'

const shop = useShopping()
const pantry = usePantry()
const movedMsg = ref('')
const week = ref(mondayOf(new Date()))
const error = ref('')
const loadError = ref('')
const loading = ref(false)
const busy = ref(false)
const name = ref('')
const quantity = ref<number | null>(null)
const unit = ref('')

const dates = computed(() => weekDates(week.value))
// Controls are only live once the list for the displayed week has actually loaded.
const ready = computed(() => !loading.value && !loadError.value && shop.week === week.value)

async function load() {
  const requested = week.value
  loadError.value = ''
  error.value = ''
  loading.value = true
  try {
    await shop.load(requested)
  } catch (e) {
    if (requested === week.value) loadError.value = errorText(e)
  } finally {
    if (requested === week.value) loading.value = false
  }
}

onMounted(load)

async function go(n: number) {
  movedMsg.value = ''
  week.value = shiftWeek(week.value, n)
  await load()
}

async function run(fn: () => Promise<void>) {
  error.value = ''
  busy.value = true
  try {
    await fn()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}

const generate = () => {
  const requested = week.value
  movedMsg.value = ''
  return run(async () => {
    await shop.generate(requested)
    if (requested === week.value) loadError.value = ''
  })
}
const movable = computed(() => shop.items.some((i) => i.checked && !i.moved && (i.quantity ?? 0) > 0))

function moveBought() {
  if (busy.value || !movable.value) return
  const requested = week.value
  return run(async () => {
    movedMsg.value = ''
    const n = await shop.moveToPantry(requested)
    if (requested === week.value) movedMsg.value = movedText(n)
    pantry.load().catch(() => { pantry.loaded = false })
  })
}
const toggle = (id: string, v: boolean | null) => run(() => shop.setChecked(id, !!v))
const remove = (id: string) => run(() => shop.remove(id))

function add() {
  const n = name.value.trim()
  if (!n || !ready.value) return
  const q = (quantity.value as unknown) === '' ? null : quantity.value
  return run(async () => {
    await shop.addManual(n, q, unit.value.trim())
    name.value = ''
    quantity.value = null
    unit.value = ''
  })
}
</script>

<template>
  <div class="d-flex align-center flex-wrap ga-2 mb-4">
    <div style="min-width: 0">
      <h1 class="text-h5">{{ t.shopping }}</h1>
      <div class="d-flex align-center week-nav">
        <v-btn icon="mdi-chevron-left" variant="text" size="small" :title="t.prevWeek" :aria-label="t.prevWeek" @click="go(-1)" />
        <span class="text-body-1">{{ formatDay(dates[0]) }} – {{ formatDay(dates[6]) }}</span>
        <v-btn icon="mdi-chevron-right" variant="text" size="small" :title="t.nextWeek" :aria-label="t.nextWeek" @click="go(1)" />
      </div>
    </div>
    <v-spacer />
    <div class="d-flex flex-wrap ga-2">
      <v-btn v-if="movable" variant="tonal" :disabled="!ready || busy" prepend-icon="mdi-package-down" @click="moveBought">
        {{ t.moveToPantry }}
      </v-btn>
      <v-btn color="primary" :loading="busy" :disabled="!ready" prepend-icon="mdi-cart-arrow-down" @click="generate">
        {{ shop.items.length ? t.regenerate : t.generate }}
      </v-btn>
    </div>
  </div>
  <v-alert v-if="movedMsg" type="success" variant="tonal" class="mb-4" closable @click:close="movedMsg = ''">{{ movedMsg }}</v-alert>
  <v-alert v-if="ready && shop.previousWeekUndone > 0" type="warning" variant="tonal" class="mb-4" data-testid="undone-warning">{{ undonePreviousText(shop.previousWeekUndone) }}</v-alert>
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>

  <v-alert v-if="loadError" type="error" variant="tonal" class="mb-4">
    {{ loadError }}
    <template #append><v-btn size="small" variant="text" @click="load">{{ t.retry }}</v-btn></template>
  </v-alert>
  <p v-else-if="!ready" class="text-medium-emphasis">{{ t.loading }}</p>

  <template v-if="ready">
    <v-form class="d-flex flex-wrap ga-2 align-start mb-4" @submit.prevent="add">
      <v-text-field v-model="name" :label="t.itemName" density="compact" hide-details style="flex: 1 1 260px" maxlength="200" />
      <v-text-field v-model.number="quantity" :label="t.quantity" type="number" min="0" step="any" density="compact" hide-details style="flex: 1 1 90px; max-width: 140px" />
      <v-text-field v-model="unit" :label="t.unit" density="compact" hide-details style="flex: 1 1 90px; max-width: 140px" />
      <v-btn type="submit" variant="tonal" :disabled="!name.trim() || busy">{{ t.addManualItem }}</v-btn>
    </v-form>

    <EmptyState v-if="!shop.items.length" :title="t.noItemsTitle" :hint="t.noItems" />
    <v-list v-else border rounded>
      <v-list-item v-for="i in shop.open" :key="i.id">
        <template #prepend>
          <v-checkbox-btn :model-value="i.checked" :aria-label="i.name" @update:model-value="toggle(i.id, $event)" />
        </template>
        <v-list-item-title>{{ i.name }}</v-list-item-title>
        <v-list-item-subtitle>
          {{ shoppingLine(i) }}<template v-if="i.manual"> · {{ t.manualBadge }}</template>
          <v-chip v-if="i.moved" size="x-small" class="ml-1">{{ t.inPantryBadge }}</v-chip>
        </v-list-item-subtitle>
        <template v-if="i.manual" #append>
          <v-btn icon="mdi-close" size="small" variant="text" :title="t.remove" @click="remove(i.id)" />
        </template>
      </v-list-item>
      <v-list-subheader v-if="shop.done.length" class="list-split">{{ t.bought }}</v-list-subheader>
      <v-list-item v-for="i in shop.done" :key="i.id" class="text-medium-emphasis">
        <template #prepend>
          <v-checkbox-btn :model-value="i.checked" :aria-label="i.name" @update:model-value="toggle(i.id, $event)" />
        </template>
        <v-list-item-title style="text-decoration: line-through">{{ i.name }}</v-list-item-title>
        <v-list-item-subtitle>
          {{ shoppingLine(i) }}
          <v-chip v-if="i.moved" size="x-small" class="ml-1">{{ t.inPantryBadge }}</v-chip>
        </v-list-item-subtitle>
        <template v-if="i.manual" #append>
          <v-btn icon="mdi-close" size="small" variant="text" :title="t.remove" @click="remove(i.id)" />
        </template>
      </v-list-item>
      <v-list-subheader v-if="shop.covered.length" class="list-split">{{ t.coveredHeader }}</v-list-subheader>
      <v-list-item v-for="i in shop.covered" :key="i.id">
        <v-list-item-title>{{ i.name }}</v-list-item-title>
        <v-list-item-subtitle>{{ shoppingLine(i) }}</v-list-item-subtitle>
        <template v-if="i.manual" #append>
          <v-btn icon="mdi-close" size="small" variant="text" :title="t.remove" @click="remove(i.id)" />
        </template>
      </v-list-item>
    </v-list>
  </template>
</template>

<style scoped>
.week-nav { margin-left: -8px; }
.list-split { border-top: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); margin-top: 4px; }
</style>
