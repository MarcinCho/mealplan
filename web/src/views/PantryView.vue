<script setup lang="ts">
import { intl } from '../i18n'
import { computed, onMounted, ref } from 'vue'
import type { PantryItem } from '../api/types'
import EmptyState from '../components/EmptyState.vue'
import PantryAddDialog from '../components/PantryAddDialog.vue'
import PantryReview from '../components/PantryReview.vue'
import { pantryErrorText, t } from '../messages'
import { formatStock, isExpired, stepFor, validExpiry } from '../pantry'
import { usePantry } from '../stores/pantry'
import { shortDate } from '../polish'
import { isoDate } from '../weeks'

const pantry = usePantry()
const q = ref<string | null>('')
const error = ref('')
const adding = ref(false)
const reviewing = ref(false)
const now = new Date()
// Staple, expiry and delete are edited rarely, so they sit behind the pencil of each card.
const editing = ref<Record<string, boolean>>({})
const soonLimit = isoDate(new Date(now.getFullYear(), now.getMonth(), now.getDate() + 2))
const expiringSoon = (d: string) => d !== '' && !isExpired(d, now) && d <= soonLimit

const shown = computed(() => {
  const needle = (q.value ?? '').trim().toLocaleLowerCase('pl-PL')
  const list = needle ? pantry.items.filter((i) => i.name.toLocaleLowerCase('pl-PL').includes(needle)) : pantry.items
  return [...list].sort((a, b) => a.name.localeCompare(b.name, 'pl'))
})

const lastReview = computed(() =>
  pantry.reviewedAt ? `${t.pantryLastReview}: ${new Date(pantry.reviewedAt).toLocaleDateString(intl)}` : t.pantryNeverReviewed,
)

async function run(fn: () => Promise<unknown>) {
  error.value = ''
  try {
    await fn()
  } catch (e) {
    error.value = pantryErrorText(e)
  }
}

const shownError = computed(() => error.value || pantry.saveError)
const load = () => run(() => pantry.load())
onMounted(load)

const stapleOf = (i: PantryItem) => (i.id in pantry.shownStaple ? pantry.shownStaple[i.id] : i.staple)
const qtyOf = (i: PantryItem) => {
  const q = i.id in pantry.shownQty ? pantry.shownQty[i.id] : i.quantity
  if (stapleOf(i)) return null
  return q ?? 0
}
const stockText = (i: PantryItem) => formatStock({ unit: i.unit, quantity: qtyOf(i), staple: stapleOf(i) })
const expiryOf = (i: PantryItem) => (i.id in pantry.shownExpiry ? pantry.shownExpiry[i.id] : i.expiresOn)

function change(i: PantryItem, dir: 1 | -1) {
  const next = Math.max(0, Math.round(((qtyOf(i) ?? 0) + dir * stepFor(i.unit)) * 1000) / 1000)
  pantry.queue(i.id, { quantity: next })
}

const toggleStaple = (i: PantryItem, staple: boolean | null) => pantry.queue(i.id, { staple: Boolean(staple) })

// Typing only updates the draft. A complete valid date is saved on change; an invalid draft is kept
// while typing (Chrome emits change for 0002-..., 0020-...) and reverted on blur.
function onExpiryInput(i: PantryItem, v: string | null) {
  pantry.shownExpiry[i.id] = v ?? ''
}
function commitExpiry(i: PantryItem, final: boolean) {
  const v = pantry.shownExpiry[i.id]
  if (v === undefined) return
  if (!validExpiry(v)) {
    if (final) delete pantry.shownExpiry[i.id]
    return
  }
  if (v === i.expiresOn) {
    delete pantry.shownExpiry[i.id]
    return
  }
  pantry.queue(i.id, { expiresOn: v })
}
const remove = (id: string) =>
  run(async () => {
    await pantry.remove(id) // cancels pending saves first
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
  <v-alert v-if="shownError" type="error" variant="tonal" class="mb-4">
    {{ shownError }}
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
        <div
          v-if="expiryOf(i) || stapleOf(i)"
          class="text-body-2"
          :class="isExpired(expiryOf(i), now) ? 'text-error font-weight-bold' : expiringSoon(expiryOf(i)) ? 'text-warning font-weight-bold' : 'text-medium-emphasis'"
        >
          <v-icon v-if="isExpired(expiryOf(i), now)" icon="mdi-alert-circle-outline" size="small" />
          <template v-if="expiryOf(i)">{{ isExpired(expiryOf(i), now) ? t.pantryExpired : t.pantryExpiresShort }} {{ shortDate(expiryOf(i), now) }}</template>
        </div>
      </div>
      <template v-if="!stapleOf(i)">
        <v-btn icon="mdi-minus" size="small" variant="tonal" :title="t.pantryDecrease" :aria-label="`${t.pantryDecrease}: ${i.name}`" :disabled="(qtyOf(i) ?? 0) <= 0" @click="change(i, -1)" />
        <span class="qty">{{ stockText(i) }}</span>
        <v-btn icon="mdi-plus" size="small" variant="tonal" :title="t.pantryIncrease" :aria-label="`${t.pantryIncrease}: ${i.name}`" @click="change(i, 1)" />
      </template>
      <span v-else class="qty">{{ stockText(i) }}</span>
      <v-btn
        icon="mdi-pencil-outline"
        size="small"
        :variant="editing[i.id] ? 'tonal' : 'text'"
        :title="t.pantryEditItem"
        :aria-label="`${t.pantryEditItem}: ${i.name}`"
        :aria-expanded="Boolean(editing[i.id])"
        @click="editing[i.id] = !editing[i.id]"
      />
    </div>
    <div v-if="editing[i.id]" class="d-flex align-center flex-wrap ga-2 mt-3">
      <v-switch :model-value="stapleOf(i)" :label="t.pantryStapleSwitch" density="compact" color="primary" hide-details class="flex-grow-0" @update:model-value="toggleStaple(i, $event)" />
      <v-text-field
        :model-value="expiryOf(i)"
        type="date"
        :label="t.pantryExpires"
        density="compact"
        hide-details
        clearable
        class="expiry"
        @update:model-value="onExpiryInput(i, $event)"
        @blur="commitExpiry(i, true)"
        @change="commitExpiry(i, false)"
        @click:clear="onExpiryInput(i, ''); commitExpiry(i, true)"
      />
      <v-spacer />
      <v-btn variant="text" color="error" prepend-icon="mdi-delete-outline" @click="remove(i.id)">{{ t.delete }}</v-btn>
    </div>
    <v-alert v-if="pantry.zeroId === i.id" type="warning" variant="tonal" density="compact" class="mt-2">
      {{ t.pantryZeroTitle }}
      <template #append>
        <v-btn size="small" color="error" variant="text" @click="remove(i.id)">{{ t.pantryDeleteAtZero }}</v-btn>
        <v-btn size="small" variant="text" @click="pantry.zeroId = null">{{ t.pantryKeepZero }}</v-btn>
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
