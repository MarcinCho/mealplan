<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import EmptyState from '../components/EmptyState.vue'
import ProductDialog from '../components/ProductDialog.vue'
import ProductPicker from '../components/ProductPicker.vue'
import ProductSuggestion from '../components/ProductSuggestion.vue'
import type { IngredientCandidate, Product, Suggestion } from '../api/types'
import { errorText, t } from '../messages'
import { useAssistant } from '../stores/assistant'
import { useNotice } from '../stores/notice'
import { useProducts } from '../stores/products'

const BATCH = 25
const MAX_TITLES = 3

interface Row {
  cand: IngredientCandidate
  sg: Suggestion
}

const assistant = useAssistant()
const products = useProducts()
const notice = useNotice()

const rows = ref<Row[]>([])
const total = ref(0)
const checked = ref(0)
const checking = ref(false)
const noSuggestion = ref(0)
const error = ref('')
const busy = ref('') // name of the row being assigned
const bulk = ref(false)
const picking = ref<string | null>(null) // name of the row showing its product picker
const creating = ref<Row | null>(null)
const creatingOpen = ref(false)
let stopped = false
let unmounted = false

const approvable = computed(() => rows.value.filter((r) => r.sg.kind === 'existing' && r.sg.productId))
const working = computed(() => checking.value || bulk.value || Boolean(busy.value))

function currentName(c: IngredientCandidate): string {
  return c.currentProductId ? (products.catalog.find((p) => p.id === c.currentProductId)?.name ?? '') : ''
}

function recipesLine(c: IngredientCandidate): string {
  const shown = c.recipeTitles.slice(0, MAX_TITLES).join(', ')
  const rest = c.recipeTitles.length - MAX_TITLES
  return rest > 0 ? `${shown} ${t.checkMoreRecipes(rest)}` : shown
}

async function run() {
  error.value = ''
  const cands = await assistant.candidates()
  total.value = cands.length
  if (!cands.length) return
  checking.value = true
  try {
    for (let i = 0; i < cands.length && !stopped && !unmounted; i += BATCH) {
      const batch = cands.slice(i, i + BATCH)
      const res = await assistant.matchIngredients(batch.map((c) => ({ name: c.name })))
      if (unmounted) return
      for (const c of batch) {
        const sg = res.find((s) => s.name === c.name)
        if (!sg || sg.kind === 'none') noSuggestion.value++
        // Pointing at the product it already has changes nothing.
        else if (sg.kind === 'existing' && sg.productId === c.currentProductId) continue
        else rows.value.push({ cand: c, sg })
      }
      checked.value = Math.min(i + BATCH, cands.length)
    }
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
  }
})
onBeforeUnmount(() => {
  unmounted = true
})

function stop() {
  stopped = true
}

// Returns whether the assignment went through.
async function assignTo(row: Row, productId: string): Promise<boolean> {
  try {
    const res = await assistant.assign(productId, row.cand.name, row.cand.recipeIds)
    rows.value = rows.value.filter((r) => r !== row)
    const msg = t.checkAssigned(row.cand.name, row.cand.recipeIds.length)
    notice.show(res.aliasConflict ? `${msg}. ${t.suggestionAliasConflict(row.cand.name, res.aliasConflict)}` : msg)
    return true
  } catch (e) {
    notice.show(errorText(e))
    return false
  }
}

async function useProduct(row: Row, productId: string) {
  busy.value = row.cand.name
  try {
    await assignTo(row, productId)
  } finally {
    busy.value = ''
  }
}

function approve(row: Row) {
  if (row.sg.kind === 'existing' && row.sg.productId) void useProduct(row, row.sg.productId)
  else if (row.sg.kind === 'new' && row.sg.newProduct) {
    creating.value = row
    creatingOpen.value = true
  }
}

function newProductCreated(p: Product) {
  products.addToCatalog(p)
  if (creating.value) void useProduct(creating.value, p.id)
}

function reject(row: Row) {
  rows.value = rows.value.filter((r) => r !== row)
}

function pickOther(row: Row, id: string | null) {
  if (!id) return
  picking.value = null
  void useProduct(row, id)
}

async function approveAll() {
  bulk.value = true
  try {
    for (const row of [...approvable.value]) {
      busy.value = row.cand.name
      if (!(await assignTo(row, row.sg.productId!))) break
    }
  } finally {
    busy.value = ''
    bulk.value = false
  }
}
</script>

<template>
  <div class="d-flex align-center flex-wrap ga-2 mb-4">
    <h1 class="text-h5 mr-auto">{{ t.checkRecipes }}</h1>
    <v-btn color="primary" :disabled="checking || bulk || !approvable.length" :loading="bulk" @click="approveAll">{{ t.checkApproveAll }}</v-btn>
  </div>
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>

  <div v-if="checking" class="mb-4">
    <div class="d-flex align-center ga-2 mb-1">
      <span class="text-body-2 mr-auto">{{ t.checkProgress(checked, total) }}</span>
      <v-btn size="small" variant="text" @click="stop">{{ t.checkStop }}</v-btn>
    </div>
    <v-progress-linear :model-value="total ? (checked / total) * 100 : 0" :aria-label="t.checkProgress(checked, total)" />
  </div>

  <EmptyState v-if="!checking && !error && !total" :title="t.checkAllMatched" />

  <v-card v-if="rows.length" class="mb-3">
    <v-list role="none">
      <v-list-item v-for="row in rows" :key="row.cand.name" class="py-3">
        <div class="d-flex align-center flex-wrap ga-2">
          <span class="text-subtitle-1 font-weight-medium">{{ row.cand.name }}</span>
          <v-chip v-if="currentName(row.cand)" size="x-small" variant="tonal" color="warning">{{ t.checkCurrent(currentName(row.cand)) }}</v-chip>
        </div>
        <div class="text-caption text-medium-emphasis mb-1">{{ recipesLine(row.cand) }}</div>
        <ProductSuggestion :suggestion="row.sg" :busy="busy === row.cand.name" @accept="approve(row)" @reject="reject(row)" />
        <v-btn v-if="picking !== row.cand.name" size="small" variant="text" :disabled="working" @click="picking = row.cand.name">{{ t.checkOtherProduct }}</v-btn>
        <ProductPicker v-else :model-value="null" allow-create :suggest-name="row.cand.name" @update:model-value="pickOther(row, $event)" />
      </v-list-item>
    </v-list>
  </v-card>
  <div v-if="noSuggestion" class="text-body-2 text-medium-emphasis">{{ t.checkNoSuggestion(noSuggestion) }}</div>

  <ProductDialog v-model="creatingOpen" :product="null" :initial-name="creating?.cand.name" :initial="creating?.sg.newProduct" @saved="newProductCreated" />
</template>
