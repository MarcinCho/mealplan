<script setup lang="ts">
import { ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { VBottomSheet, VDialog } from 'vuetify/components'
import type { Product } from '../api/types'
import { productErrorText, t } from '../messages'
import { emptyProduct, useProducts } from '../stores/products'

const props = defineProps<{ modelValue: boolean; product: Product | null; initialName?: string }>()
const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'saved', p: Product): void
}>()

const { mdAndUp } = useDisplay()
const products = useProducts()
const draft = ref<Product>(emptyProduct())
const error = ref('')
const busy = ref(false)

watch(
  () => [props.modelValue, props.product] as const,
  () => {
    if (!props.modelValue) return
    error.value = ''
    const p = props.product
    draft.value = p
      ? { ...p, per100g: { ...p.per100g }, aliases: [...p.aliases] }
      : { ...emptyProduct(), name: props.initialName ?? '' }
  },
  { immediate: true },
)

async function save() {
  error.value = ''
  busy.value = true
  try {
    const saved = await products.save(draft.value)
    emit('saved', saved)
    emit('update:modelValue', false)
  } catch (e) {
    error.value = productErrorText(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <component
    :is="mdAndUp ? VDialog : VBottomSheet"
    :model-value="modelValue"
    max-width="520"
    scrollable
    :aria-label="product ? t.editProduct : t.newProduct"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <v-card>
      <v-card-title>{{ product ? t.editProduct : t.newProduct }}</v-card-title>
      <v-card-text>
        <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
        <v-text-field v-model="draft.name" :label="t.productName" maxlength="120" autofocus />
        <div class="text-subtitle-2 mb-2">{{ t.per100g }}</div>
        <v-row dense>
          <v-col cols="6"><v-text-field v-model.number="draft.per100g.kcal" :label="t.kcalShort" type="number" min="0" inputmode="decimal" /></v-col>
          <v-col cols="6"><v-text-field v-model.number="draft.per100g.protein" :label="t.proteinShort" type="number" min="0" inputmode="decimal" /></v-col>
          <v-col cols="6"><v-text-field v-model.number="draft.per100g.carbs" :label="t.carbsShort" type="number" min="0" inputmode="decimal" /></v-col>
          <v-col cols="6"><v-text-field v-model.number="draft.per100g.fat" :label="t.fatShort" type="number" min="0" inputmode="decimal" /></v-col>
          <v-col cols="6"><v-text-field v-model.number="draft.pieceGrams" :label="t.pieceGrams" type="number" min="0" inputmode="decimal" /></v-col>
          <v-col cols="6"><v-text-field v-model.number="draft.densityGPerMl" :label="t.density" type="number" min="0" inputmode="decimal" /></v-col>
        </v-row>
        <v-combobox
          v-model="draft.aliases"
          :label="t.aliases"
          :hint="t.aliasesHint"
          persistent-hint
          multiple
          chips
          closable-chips
        />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="emit('update:modelValue', false)">{{ t.cancel }}</v-btn>
        <v-btn color="primary" :disabled="!(draft.name ?? '').trim() || busy" :loading="busy" @click="save">{{ t.save }}</v-btn>
      </v-card-actions>
    </v-card>
  </component>
</template>
