<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import EmptyState from '../components/EmptyState.vue'
import type { ProductUsage } from '../api/types'
import { ApiError } from '../api/client'
import { errorText, t, mergedNotice, usageLine } from '../messages'
import { useAssistant } from '../stores/assistant'
import { useNotice } from '../stores/notice'
import { useProducts } from '../stores/products'

interface Row {
  key: string
  keep: string // the product that stays; the others are merged into it
  off: string[] // ids the user unchecked: they are left out of the merge
  ids: string[]
  reason: string
}

const assistant = useAssistant()
const products = useProducts()
const notice = useNotice()

const rows = ref<Row[]>([])
const usages = ref<Record<string, ProductUsage>>({})
const total = ref(0)
const checked = ref(0)
const loading = ref(true)
const checking = ref(false)
const finished = ref(false)
const error = ref('')
const busy = ref('') // key of the row being merged
const absorbed = new Set<string>() // merged away in this session; later batches may still mention them
let stopped = false
let unmounted = false
let n = 0

const working = computed(() => checking.value || Boolean(busy.value))

function nameOf(id: string): string {
  return products.catalog.find((p) => p.id === id)?.name ?? ''
}

// Drops products that no longer exist; a row left with fewer than two is not a duplicate group any more.
function prune() {
  rows.value = rows.value
    .map((r) => {
      const ids = r.ids.filter((id) => !absorbed.has(id))
      return { ...r, ids, keep: ids.includes(r.keep) ? r.keep : ids[0] }
    })
    .filter((r) => r.ids.length >= 2)
}

async function loadUsage(ids: string[]) {
  await Promise.all(
    ids.map(async (id) => {
      try {
        usages.value[id] = await assistant.usage(id)
      } catch {
        // the counts are only a hint for choosing; the group is still usable without them
      }
    }),
  )
}

async function run() {
  const batches = await assistant.productBatches()
  loading.value = false
  total.value = batches.length
  if (!batches.length) return
  checking.value = true
  try {
    for (let i = 0; i < batches.length && !stopped && !unmounted; i++) {
      const groups = await assistant.duplicates(batches[i])
      if (unmounted) return
      for (const g of groups) {
        const ids = [g.keepId, ...g.absorbIds].filter((id, j, all) => !absorbed.has(id) && all.indexOf(id) === j && nameOf(id))
        if (ids.length < 2) continue
        rows.value.push({ key: String(n++), off: [], keep: ids.includes(g.keepId) ? g.keepId : ids[0], ids, reason: g.reason })
        void loadUsage(ids)
      }
      checked.value = i + 1
    }
    finished.value = !stopped
  } finally {
    checking.value = false
  }
}

onMounted(async () => {
  try {
    await Promise.all([assistant.loaded ? Promise.resolve() : assistant.load(), products.ensureLoaded()])
    await run()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    loading.value = false
  }
})
onBeforeUnmount(() => {
  unmounted = true
})

function stop() {
  stopped = true
}

const absorbOf = (row: Row) => row.ids.filter((id) => id !== row.keep && !row.off.includes(id))

function setKeep(row: Row, id: string) {
  row.keep = id
  row.off = row.off.filter((x) => x !== id) // the product that stays is never left out of anything
}

function setIncluded(row: Row, id: string, on: boolean) {
  row.off = on ? row.off.filter((x) => x !== id) : [...row.off, id]
}

function dismiss(row: Row) {
  rows.value = rows.value.filter((r) => r !== row)
}

async function merge(row: Row) {
  const absorb = absorbOf(row)
  if (!absorb.length) return
  busy.value = row.key
  try {
    const res = await assistant.merge(row.keep, absorb)
    absorb.forEach((id) => absorbed.add(id))
    notice.show(mergedNotice(nameOf(row.keep), res.skippedAliases))
    dismiss(row)
    prune()
    await products.merged(absorb).catch(() => {}) // the merge is done; a failed refetch only leaves the catalog stale
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) {
      // A product of this group is already gone (merged by an earlier group or removed elsewhere).
      notice.show(t.productAlreadyMerged)
      dismiss(row)
      await products.ensureLoaded(true).catch(() => {})
    } else {
      notice.show(errorText(e))
    }
  } finally {
    busy.value = ''
  }
}
</script>

<template>
  <div class="d-flex align-center flex-wrap ga-2 mb-4">
    <h1 class="text-h5 mr-auto">{{ t.checkProducts }}</h1>
  </div>
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>

  <div v-if="checking" class="mb-4">
    <div class="d-flex align-center ga-2 mb-1">
      <span class="text-body-2 mr-auto">{{ t.checkProgress(checked, total) }}</span>
      <v-btn size="small" variant="text" @click="stop">{{ t.checkStop }}</v-btn>
    </div>
    <v-progress-linear :model-value="total ? (checked / total) * 100 : 0" :aria-label="t.checkProgress(checked, total)" />
    <div v-if="assistant.waitingSeconds > 0" class="text-caption text-medium-emphasis mt-1" data-test="waiting">{{ t.checkWaiting(assistant.waitingSeconds) }}</div>
  </div>

  <v-progress-linear v-if="loading" indeterminate :aria-label="t.checkProducts" class="mb-4" />
  <EmptyState v-else-if="!error && !total" :title="t.productCheckNoBatches" />
  <EmptyState v-else-if="finished && !rows.length" :title="t.checkNothingToChange" />

  <v-card v-for="row in rows" :key="row.key" class="mb-3">
    <v-card-text>
      <div class="text-caption text-medium-emphasis mb-1">{{ row.reason }}</div>
      <div class="text-body-2 mb-1">{{ t.productCheckKeepHint }}</div>
      <!-- One radio per group: a checkbox cannot sit inside a v-radio-group (it would join the group), and the keep state lives in row.keep anyway. -->
      <div v-for="id in row.ids" :key="id" class="d-flex align-center">
        <v-radio-group :model-value="row.keep" hide-details :disabled="working" class="flex-grow-1" @update:model-value="setKeep(row, $event)">
          <v-radio :value="id">
            <template #label>
              <div>
                <div>{{ nameOf(id) }}</div>
                <div class="text-caption text-medium-emphasis">{{ usages[id] ? usageLine(usages[id]) : '' }}</div>
              </div>
            </template>
          </v-radio>
        </v-radio-group>
        <v-checkbox
          v-if="id !== row.keep"
          :model-value="!row.off.includes(id)"
          :aria-label="t.productCheckInclude(nameOf(id))"
          hide-details
          density="compact"
          class="flex-grow-0"
          :disabled="working"
          @update:model-value="setIncluded(row, id, Boolean($event))"
        />
      </div>
    </v-card-text>
    <v-card-actions>
      <v-spacer />
      <v-btn variant="text" :disabled="working" @click="dismiss(row)">{{ t.suggestionReject }}</v-btn>
      <v-btn color="primary" :disabled="working || !absorbOf(row).length" :loading="busy === row.key" @click="merge(row)">{{ t.productCheckMerge }}</v-btn>
    </v-card-actions>
  </v-card>
</template>
