<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import EmptyState from '../components/EmptyState.vue'
import ProductDialog from '../components/ProductDialog.vue'
import type { Product } from '../api/types'
import { formatMacros } from '../nutrition'
import { productErrorText, t } from '../messages'
import { useProducts } from '../stores/products'

const products = useProducts()
const q = ref('')
const error = ref('')
const dialog = ref(false)
const editing = ref<Product | null>(null)
const removing = ref('')
let timer: ReturnType<typeof setTimeout> | undefined

async function load() {
  try {
    await products.load(q.value.trim() || undefined)
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

onMounted(load)
onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <div class="d-flex align-center mb-4">
    <h1 class="text-h5">{{ t.products }}</h1>
    <v-spacer />
    <v-btn color="primary" prepend-icon="mdi-plus" @click="open(null)">{{ t.newProduct }}</v-btn>
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
</template>

<style scoped>
.wrap { white-space: normal; display: block; }
</style>
