<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { api } from '../api/client'
import type { Product } from '../api/types'
import { errorText, t } from '../messages'
import ProductDialog from './ProductDialog.vue'

withDefaults(defineProps<{ modelValue: string | null; allowCreate?: boolean }>(), { allowCreate: false })
const emit = defineEmits<{
  (e: 'update:modelValue', v: string | null): void
  (e: 'picked', p: Product | null): void
}>()

const items = ref<Product[]>([])
const search = ref('')
const error = ref('')
const creating = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
let seq = 0

async function fetchItems(q: string) {
  const mine = ++seq
  try {
    const res = await api<Product[]>('GET', '/products' + (q ? `?q=${encodeURIComponent(q)}` : ''))
    if (mine === seq) {
      items.value = res
      error.value = ''
    }
  } catch (e) {
    error.value = errorText(e)
  }
}

function onSearch(q: string | null) {
  search.value = q ?? ''
  clearTimeout(timer)
  timer = setTimeout(() => fetchItems(search.value), 200)
}

function pick(id: string | null) {
  emit('update:modelValue', id)
  emit('picked', items.value.find((p) => p.id === id) ?? null)
}

function created(p: Product) {
  items.value = [p, ...items.value.filter((i) => i.id !== p.id)]
  pick(p.id)
}

onBeforeUnmount(() => clearTimeout(timer))
fetchItems('')
</script>

<template>
  <v-autocomplete
    :model-value="modelValue"
    :items="items"
    item-title="name"
    item-value="id"
    :label="t.product"
    :error-messages="error"
    no-filter
    clearable
    @update:model-value="pick"
    @update:search="onSearch"
  >
    <template v-if="allowCreate" #append-item>
      <v-list-item prepend-icon="mdi-plus" :title="t.addProduct" @click="creating = true" />
    </template>
    <template v-if="allowCreate" #no-data>
      <v-list-item prepend-icon="mdi-plus" :title="t.addProduct" @click="creating = true" />
    </template>
  </v-autocomplete>
  <ProductDialog v-if="allowCreate" v-model="creating" :product="null" :initial-name="search" @saved="created" />
</template>
