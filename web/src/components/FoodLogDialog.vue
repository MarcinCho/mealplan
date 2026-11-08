<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { VBottomSheet, VDialog } from 'vuetify/components'
import type { Product } from '../api/types'
import { t } from '../messages'
import { validQuantity } from '../nutrition'
import { useProducts } from '../stores/products'
import type { LogInput } from '../stores/nutrition'
import ProductPicker from './ProductPicker.vue'

const props = defineProps<{ modelValue: boolean; date: string }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void; (e: 'save', input: LogInput): void }>()

const { mdAndUp } = useDisplay()
const products = useProducts()
const productId = ref<string | null>(null)
const picked = ref<Product | null>(null)
const quantity = ref<number | null>(null)
const unit = ref<'g' | 'szt'>('g')

watch(
  () => props.modelValue,
  (open) => {
    if (!open) return
    productId.value = null
    picked.value = null
    quantity.value = null
    unit.value = 'g'
  },
)

const product = computed(() => picked.value ?? products.catalog.find((p) => p.id === productId.value) ?? null)
const canPiece = computed(() => !!product.value?.pieceGrams)
watch(canPiece, (ok) => {
  if (!ok) unit.value = 'g'
})

const quantityOk = computed(() => validQuantity(quantity.value))
const quantityError = computed(() => (quantity.value !== null && quantity.value !== undefined && !quantityOk.value ? [t.logQuantityInvalid] : []))
const valid = computed(() => !!productId.value && quantityOk.value)

function save() {
  if (!valid.value) return
  emit('save', { date: props.date, productId: productId.value!, quantity: quantity.value!, unit: unit.value })
}
</script>

<template>
  <component
    :is="mdAndUp ? VDialog : VBottomSheet"
    :model-value="modelValue"
    max-width="480"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <v-card>
      <v-card-title>{{ t.logTitle }}</v-card-title>
      <v-card-text>
        <ProductPicker v-model="productId" allow-create @picked="picked = $event" />
        <v-text-field v-model.number="quantity" :label="t.logQuantity" type="number" min="0" max="10000" step="any" :error-messages="quantityError" />
        <v-btn-toggle v-model="unit" mandatory density="comfortable" color="primary" :aria-label="t.logUnit">
          <v-btn value="g">g</v-btn>
          <v-btn v-if="canPiece" value="szt">szt</v-btn>
        </v-btn-toggle>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="emit('update:modelValue', false)">{{ t.cancel }}</v-btn>
        <v-btn color="primary" :disabled="!valid" @click="save">{{ t.logAdd }}</v-btn>
      </v-card-actions>
    </v-card>
  </component>
</template>
