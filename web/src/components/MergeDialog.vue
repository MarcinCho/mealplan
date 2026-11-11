<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { VBottomSheet, VDialog } from 'vuetify/components'
import type { MergeResult, Product, ProductUsage } from '../api/types'
import { errorText, t, usageLine } from '../messages'
import { useAssistant } from '../stores/assistant'
import { useProducts } from '../stores/products'

// Manual merge: `keep` stays, the product picked here is absorbed into it.
const props = defineProps<{ modelValue: boolean; keep: Product | null }>()
const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'merged', keep: Product, result: MergeResult): void
}>()

const { mdAndUp } = useDisplay()
const assistant = useAssistant()
const products = useProducts()
const absorbId = ref<string | null>(null)
const usage = ref<ProductUsage | null>(null)
const error = ref('')
const busy = ref(false)
let seq = 0

const choices = computed(() => products.catalog.filter((p) => p.id !== props.keep?.id))
const absorb = computed(() => choices.value.find((p) => p.id === absorbId.value) ?? null)

watch(
  () => props.modelValue,
  (open) => {
    if (!open) return
    absorbId.value = null
    usage.value = null
    error.value = ''
    products.ensureLoaded().catch((e) => {
      error.value = errorText(e)
    })
  },
  { immediate: true },
)

watch(absorbId, async (id) => {
  usage.value = null
  if (!id) return
  const mine = ++seq
  try {
    const u = await assistant.usage(id)
    if (mine === seq) usage.value = u
  } catch (e) {
    if (mine === seq) error.value = errorText(e)
  }
})

async function confirm() {
  if (!props.keep || !absorb.value) return
  error.value = ''
  busy.value = true
  try {
    const result = await assistant.merge(props.keep.id, [absorb.value.id])
    await products.merged([absorb.value.id]).catch(() => {}) // the merge is done; a failed refetch only leaves the catalog stale
    emit('merged', props.keep, result)
    emit('update:modelValue', false)
  } catch (e) {
    error.value = errorText(e)
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
    :aria-label="t.mergeWith"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <v-card>
      <v-card-title>{{ t.mergeWith }}</v-card-title>
      <v-card-text>
        <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
        <div class="text-subtitle-1 mb-3">{{ keep?.name }}</div>
        <v-autocomplete v-model="absorbId" :items="choices" item-title="name" item-value="id" :label="t.mergeAbsorb" no-data-text="—" />
        <template v-if="absorb">
          <div class="text-body-2 text-medium-emphasis mb-2">{{ usage ? usageLine(usage) : '' }}</div>
          <v-alert type="warning" variant="tonal" density="compact">{{ t.mergeWarning(absorb.name, keep?.name ?? '') }}</v-alert>
        </template>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="emit('update:modelValue', false)">{{ t.cancel }}</v-btn>
        <v-btn color="primary" :disabled="!absorb || busy" :loading="busy" @click="confirm">{{ t.productCheckMerge }}</v-btn>
      </v-card-actions>
    </v-card>
  </component>
</template>
