<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { api } from '../api/client'
import type { Product } from '../api/types'
import { errorText, t } from '../messages'
import { useProducts } from '../stores/products'
import ProductDialog from './ProductDialog.vue'

const props = withDefaults(defineProps<{ modelValue: string | null; allowCreate?: boolean; matchedId?: string | null }>(), {
  allowCreate: false,
  matchedId: null,
})
const emit = defineEmits<{
  (e: 'update:modelValue', v: string | null): void
  (e: 'picked', p: Product | null): void
}>()

const store = useProducts()
const found = ref<Product[] | null>(null) // server search results; null while the search box is empty
const search = ref('')
const error = ref('')
const creating = ref(false)
const gone = ref(false) // the remembered product no longer exists
const resolved = ref<Product | null>(null) // current value / matched product when not in the shared list
let timer: ReturnType<typeof setTimeout> | undefined
let seq = 0

// The shared list is loaded once for all pickers on the page.
store.ensureLoaded().catch((e) => {
  error.value = errorText(e)
})

const items = computed(() => {
  const base = found.value ?? store.catalog
  const extra = [props.modelValue, props.matchedId]
    .map((id) => (id ? (store.catalog.find((p) => p.id === id) ?? (resolved.value?.id === id ? resolved.value : undefined)) : undefined))
    .filter((p): p is Product => Boolean(p) && !base.some((b) => b.id === p!.id))
  return [...extra, ...base]
})

const matched = computed(() => {
  if (props.modelValue || !props.matchedId) return null
  return items.value.find((p) => p.id === props.matchedId) ?? null
})
const placeholder = computed(() =>
  gone.value ? t.productDeleted : matched.value ? `${t.matchedProduct}: ${matched.value.name}` : '',
)

// A value missing from the list is resolved by id (one refresh), never shown raw.
watch(
  () => [props.modelValue, props.matchedId, store.catalogLoaded],
  async () => {
    for (const id of [props.modelValue, props.matchedId]) {
      if (id && !store.catalog.some((p) => p.id === id) && resolved.value?.id !== id) {
        try {
          const p = await store.resolve(id)
          if (p) resolved.value = p
          else if (id === props.modelValue) {
            // Deleted product: clear the choice so no raw id shows and it is saved as null.
            gone.value = true
            emit('update:modelValue', null)
          }
        } catch (e) {
          error.value = errorText(e)
        }
      }
    }
  },
  { immediate: true },
)

async function fetchItems(q: string) {
  const mine = ++seq
  if (!q) {
    found.value = null
    return
  }
  try {
    const res = await api<Product[]>('GET', `/products?q=${encodeURIComponent(q)}`)
    if (mine === seq) {
      found.value = res
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
  gone.value = false
  emit('update:modelValue', id)
  emit('picked', items.value.find((p) => p.id === id) ?? null)
}

function created(p: Product) {
  store.addToCatalog(p)
  pick(p.id)
}

onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <v-autocomplete
    :model-value="modelValue"
    :items="items"
    item-title="name"
    item-value="id"
    :label="t.product"
    :placeholder="placeholder"
    persistent-placeholder
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
