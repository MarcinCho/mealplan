<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import EmptyState from '../components/EmptyState.vue'
import MergeDialog from '../components/MergeDialog.vue'
import ProductDialog from '../components/ProductDialog.vue'
import type { MergeResult, Product } from '../api/types'
import { formatMacros } from '../nutrition'
import { mergedNotice, productErrorText, t } from '../messages'
import { useAssistant } from '../stores/assistant'
import { useNotice } from '../stores/notice'
import { useProducts } from '../stores/products'

const products = useProducts()
const assistant = useAssistant()
const notice = useNotice()
const merging = ref<Product | null>(null)
const mergeOpen = ref(false)
const q = ref<string | null>('')
const error = ref('')
const dialog = ref(false)
const editing = ref<Product | null>(null)
const removing = ref('')
let timer: ReturnType<typeof setTimeout> | undefined

async function load() {
  try {
    await products.load(q.value)
    error.value = ''
  } catch (e) {
    error.value = productErrorText(e)
  }
}

function onSearch() {
  clearTimeout(timer)
  timer = setTimeout(load, 250)
}

function open(p: Product | null) {
  editing.value = p
  dialog.value = true
}

async function remove(id: string) {
  error.value = ''
  try {
    await products.remove(id)
  } catch (e) {
    error.value = productErrorText(e)
  } finally {
    removing.value = ''
  }
}

function openMerge(p: Product) {
  merging.value = p
  mergeOpen.value = true
}

function merged(keep: Product, res: MergeResult) {
  notice.show(mergedNotice(keep.name, res.skippedAliases))
  void load()
}

onMounted(() => {
  void load()
  if (!assistant.loaded) assistant.load().catch(() => {}) // the check button is optional: a failed settings load just hides it
})
onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <div class="d-flex align-center flex-wrap ga-2 mb-4">
    <h1 class="text-h5 mr-auto">{{ t.products }}</h1>
    <div class="d-flex flex-wrap ga-2">
      <v-btn v-if="assistant.configured" variant="tonal" :to="{ name: 'product-check' }" prepend-icon="mdi-magnify">{{ t.checkProducts }}</v-btn>
      <v-btn color="primary" prepend-icon="mdi-plus" @click="open(null)">{{ t.newProduct }}</v-btn>
    </div>
  </div>
  <v-text-field
    v-model="q"
    :label="t.searchProducts"
    prepend-inner-icon="mdi-magnify"
    clearable
    hide-details
    class="mb-4"
    @update:model-value="onSearch"
  />
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
  <EmptyState v-if="products.loaded && !products.items.length" :title="t.noProducts" :hint="t.noProductsHint" />
  <v-list v-else lines="two" border rounded>
    <v-list-item v-for="p in products.items" :key="p.id" :title="p.name" @click="open(p)">
      <template #subtitle>
        <span class="wrap">{{ formatMacros(p.per100g) }}</span>
      </template>
      <template #append>
        <v-menu>
          <template #activator="{ props: menu }">
            <v-btn v-bind="menu" icon="mdi-dots-vertical" size="small" variant="text" :aria-label="t.moreActions" @click.stop />
          </template>
          <v-list>
            <v-list-item prepend-icon="mdi-call-merge" :title="t.mergeWith" @click="openMerge(p)" />
          </v-list>
        </v-menu>
        <v-btn
          size="small"
          variant="text"
          color="error"
          @click.stop="removing === p.id ? remove(p.id) : (removing = p.id)"
        >{{ removing === p.id ? t.confirmDeleteProduct : t.delete }}</v-btn>
      </template>
    </v-list-item>
  </v-list>
  <ProductDialog v-model="dialog" :product="editing" @saved="load" />
  <MergeDialog v-model="mergeOpen" :keep="merging" @merged="merged" />
</template>

<style scoped>
.wrap { white-space: normal; display: block; }
</style>
