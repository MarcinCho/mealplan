<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import type { PantryItem } from '../api/types'
import EmptyState from '../components/EmptyState.vue'
import PantryAddDialog from '../components/PantryAddDialog.vue'
import PantryReview from '../components/PantryReview.vue'
import { pantryErrorText, t } from '../messages'
import { createSerialSaver, formatStock, isExpired, stepFor, validExpiry, type SerialSaver } from '../pantry'
import { usePantry } from '../stores/pantry'

const pantry = usePantry()
const q = ref<string | null>('')
const error = ref('')
const adding = ref(false)
const reviewing = ref(false)
const zeroFor = ref<string | null>(null) // item that reached 0 and awaits delete/keep
const now = new Date()

const shown = computed(() => {
  const needle = (q.value ?? '').trim().toLocaleLowerCase('pl-PL')
  const list = needle ? pantry.items.filter((i) => i.name.toLocaleLowerCase('pl-PL').includes(needle)) : pantry.items
  return [...list].sort((a, b) => a.name.localeCompare(b.name, 'pl'))
})

const lastReview = computed(() =>
  pantry.reviewedAt ? `${t.pantryLastReview}: ${new Date(pantry.reviewedAt).toLocaleDateString('pl-PL')}` : t.pantryNeverReviewed,
)

async function run(fn: () => Promise<unknown>) {
  error.value = ''
  try {
    await fn()
  } catch (e) {
    error.value = pantryErrorText(e)
  }
}

const load = () => run(() => pantry.load())
onMounted(load)

interface Patch { quantity?: number; expiresOn?: string }
const shownQty = reactive<Record<string, number>>({}) // optimistic quantities while saves are pending
const shownExpiry = reactive<Record<string, string>>({})
const savers = new Map<string, SerialSaver<Patch>>()

function clearShown(id: string) {
  delete shownQty[id]
  delete shownExpiry[id]
}

function saverFor(id: string): SerialSaver<Patch> {
  let s = savers.get(id)
  if (!s) {
    s = createSerialSaver<Patch>({
      delay: 400,
      merge: (a, b) => ({ ...a, ...b }),
      send: async (patch) => {
        const cur = pantry.items.find((x) => x.id === id)
        if (!cur) return
        await pantry.update(id, {
          quantity: patch.quantity ?? cur.quantity,
          unit: cur.unit,
          expiresOn: patch.expiresOn ?? cur.expiresOn,
          staple: cur.staple,
        })
      },
      onIdle: () => {
        clearShown(id)
        const q = pantry.items.find((x) => x.id === id)?.quantity
        zeroFor.value = q === 0 ? id : zeroFor.value === id ? null : zeroFor.value
      },
      onError: (e) => {
        clearShown(id) // roll back to what the server has
        error.value = pantryErrorText(e)
      },
    })
    savers.set(id, s)
  }
  return s
}

const qtyOf = (i: PantryItem) => (i.id in shownQty ? shownQty[i.id] : i.quantity)
const stockText = (i: PantryItem) => formatStock({ ...i, quantity: qtyOf(i) })

function change(i: PantryItem, dir: 1 | -1) {
  error.value = ''
  const next = Math.max(0, Math.round(((qtyOf(i) ?? 0) + dir * stepFor(i.unit)) * 1000) / 1000)
  shownQty[i.id] = next
  saverFor(i.id).set({ quantity: next })
}

const expiryOf = (i: PantryItem) => (i.id in shownExpiry ? shownExpiry[i.id] : i.expiresOn)

// Typing only updates the draft; the save happens on change/blur and only for a sane date.
function commitExpiry(i: PantryItem) {
  const v = shownExpiry[i.id]
  if (v === undefined) return
  if (!validExpiry(v)) {
    delete shownExpiry[i.id]
    return
  }
  if (v === i.expiresOn) {
    delete shownExpiry[i.id]
    return
  }
  error.value = ''
  saverFor(i.id).set({ expiresOn: v })
}
const toggleStaple = (i: PantryItem, staple: boolean | null) =>
  run(() =>
    pantry.update(i.id, {
      quantity: staple ? null : (i.quantity ?? 0),
      unit: i.unit,
      expiresOn: i.expiresOn,
      staple: Boolean(staple),
    }),
  )
const remove = (id: string) =>
  run(async () => {
    savers.delete(id)
    clearShown(id)
    await pantry.remove(id)
    zeroFor.value = null
  })
</script>

<template>
  <div class="d-flex align-center flex-wrap ga-2 mb-2">
    <h1 class="text-h5">{{ t.pantry }}</h1>
    <v-spacer />
    <v-btn variant="tonal" prepend-icon="mdi-clipboard-check-outline" :disabled="!pantry.items.length" @click="reviewing = true">{{ t.pantryReview }}</v-btn>
    <v-btn color="primary" prepend-icon="mdi-plus" @click="adding = true">{{ t.pantryAdd }}</v-btn>
  </div>
  <div class="text-body-2 text-medium-emphasis mb-4">{{ lastReview }}</div>
  <v-text-field v-model="q" :label="t.pantrySearch" prepend-inner-icon="mdi-magnify" clearable hide-details class="mb-4" />
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">
    {{ error }}
    <template v-if="!pantry.loaded" #append><v-btn variant="text" @click="load">{{ t.retry }}</v-btn></template>
  </v-alert>
  <v-progress-linear v-if="!pantry.loaded && !error" indeterminate class="mb-4" />
  <EmptyState v-if="pantry.loaded && !pantry.items.length" :title="t.pantryEmpty" :hint="t.pantryEmptyHint">
    <v-btn color="primary" @click="adding = true">{{ t.pantryAdd }}</v-btn>
  </EmptyState>
  <div v-else-if="pantry.loaded && !shown.length" class="text-medium-emphasis">{{ t.pantryNoMatches }}</div>

  <v-card v-for="i in shown" :key="i.id" class="mb-2 pa-3">
    <div class="d-flex align-center ga-2">
      <div class="flex-grow-1 name-col">
        <div class="font-weight-bold item-name">{{ i.name }}</div>
        <div class="text-body-2" :class="isExpired(expiryOf(i), now) ? 'text-error font-weight-bold' : 'text-medium-emphasis'">
          <v-icon v-if="isExpired(expiryOf(i), now)" icon="mdi-alert-circle-outline" size="small" />
          <template v-if="expiryOf(i)">{{ isExpired(expiryOf(i), now) ? t.pantryExpired : t.pantryExpiresShort }} {{ expiryOf(i) }}</template>
        </div>
      </div>
      <template v-if="!i.staple">
        <v-btn icon="mdi-minus" size="small" variant="tonal" :title="t.pantryDecrease" :disabled="(qtyOf(i) ?? 0) <= 0" @click="change(i, -1)" />
        <span class="qty">{{ stockText(i) }}</span>
        <v-btn icon="mdi-plus" size="small" variant="tonal" :title="t.pantryIncrease" @click="change(i, 1)" />
      </template>
      <span v-else class="qty">{{ stockText(i) }}</span>
    </div>
    <div class="d-flex align-center flex-wrap ga-2 mt-2">
      <v-switch :model-value="i.staple" :label="t.pantryStapleSwitch" density="compact" color="primary" hide-details @update:model-value="toggleStaple(i, $event)" />
      <v-text-field
        :model-value="expiryOf(i)"
        type="date"
        :label="t.pantryExpires"
        density="compact"
        hide-details
        clearable
        class="expiry"
        @update:model-value="shownExpiry[i.id] = $event ?? ''"
        @blur="commitExpiry(i)"
        @change="commitExpiry(i)"
        @click:clear="shownExpiry[i.id] = ''; commitExpiry(i)"
      />
      <v-spacer />
      <v-btn icon="mdi-delete-outline" variant="text" color="error" size="small" :title="t.delete" @click="remove(i.id)" />
    </div>
    <v-alert v-if="zeroFor === i.id" type="warning" variant="tonal" density="compact" class="mt-2">
      {{ t.pantryZeroTitle }}
      <template #append>
        <v-btn size="small" color="error" variant="text" @click="remove(i.id)">{{ t.pantryDeleteAtZero }}</v-btn>
        <v-btn size="small" variant="text" @click="zeroFor = null">{{ t.pantryKeepZero }}</v-btn>
      </template>
    </v-alert>
  </v-card>

  <PantryAddDialog v-model="adding" />
  <PantryReview v-model="reviewing" />
</template>

<style scoped>
.name-col { min-width: 0; }
.item-name { overflow-wrap: anywhere; }
.qty { min-width: 64px; text-align: center; font-weight: 600; }
.expiry { max-width: 190px; min-width: 150px; }
</style>
