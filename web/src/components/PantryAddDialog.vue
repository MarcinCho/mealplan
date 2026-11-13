<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { VBottomSheet, VDialog } from 'vuetify/components'
import { pantryErrorText, t } from '../messages'
import { PANTRY_UNITS } from '../pantry'
import { usePantry } from '../stores/pantry'
import ProductPicker from './ProductPicker.vue'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void; (e: 'added'): void }>()

const { mdAndUp } = useDisplay()
const pantry = usePantry()
const productId = ref<string | null>(null)
const quantity = ref<number | string | null>(null)
const unit = ref('g')
const expiresOn = ref('')
const staple = ref(false)
const error = ref('')
const busy = ref(false)

watch(
  () => props.modelValue,
  (open) => {
    if (!open) return
    productId.value = null
    quantity.value = null
    unit.value = 'g'
    expiresOn.value = ''
    staple.value = false
    error.value = ''
  },
)

const qtyOk = computed(() => {
  if (staple.value) return true
  const q = quantity.value
  return q !== null && q !== '' && Number.isFinite(Number(q)) && Number(q) >= 0 && Number(q) <= 1000000
})
const valid = computed(() => !!productId.value && qtyOk.value)

async function submit() {
  if (!valid.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    await pantry.add({
      productId: productId.value!,
      quantity: staple.value ? null : Number(quantity.value),
      unit: unit.value,
      expiresOn: expiresOn.value || '',
      staple: staple.value,
    })
    emit('added')
    emit('update:modelValue', false)
  } catch (e) {
    error.value = pantryErrorText(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <component :is="mdAndUp ? VDialog : VBottomSheet" :model-value="modelValue" max-width="480" :aria-label="t.pantryAddTitle" @update:model-value="emit('update:modelValue', $event)">
    <v-card>
      <v-card-title>{{ t.pantryAddTitle }}</v-card-title>
      <v-card-text>
        <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
        <ProductPicker v-model="productId" allow-create />
        <div class="d-flex ga-3">
          <v-text-field
            v-model.number="quantity"
            :label="t.quantity"
            type="number"
            min="0"
            inputmode="decimal"
            :disabled="staple"
            :error="!qtyOk"
            :hint="qtyOk ? '' : t.pantryQuantityRequired"
            class="flex-grow-1"
            style="min-width: 0"
          />
          <v-select v-model="unit" :items="PANTRY_UNITS" :label="t.unit" style="max-width: 150px; min-width: 110px" />
        </div>
        <v-text-field v-model="expiresOn" :label="t.pantryExpires" type="date" clearable />
        <v-switch v-model="staple" :label="t.pantryStapleSwitch" color="primary" hide-details />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="emit('update:modelValue', false)">{{ t.cancel }}</v-btn>
        <v-btn color="primary" :disabled="!valid" :loading="busy" @click="submit">{{ t.pantryAdd }}</v-btn>
      </v-card-actions>
    </v-card>
  </component>
</template>
