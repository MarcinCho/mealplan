<script setup lang="ts">
import type { Suggestion } from '../api/types'
import { t } from '../messages'

defineProps<{ suggestion: Suggestion; busy?: boolean; disabled?: boolean }>()
defineEmits<{ (e: 'accept'): void; (e: 'reject'): void }>()
</script>

<template>
  <div class="suggestion d-flex align-center flex-wrap ga-2 pb-2">
    <v-icon icon="mdi-magnify" size="small" />
    <span class="text-body-2 flex-grow-1">
      {{ suggestion.kind === 'new' ? t.suggestionNew(suggestion.newProduct?.name ?? suggestion.name) : t.suggestionExisting(suggestion.productName ?? '') }}
    </span>
    <v-btn size="small" color="primary" variant="tonal" :loading="busy" :disabled="disabled" @click="$emit('accept')">{{ t.suggestionAccept }}</v-btn>
    <v-btn size="small" variant="text" :disabled="busy || disabled" @click="$emit('reject')">{{ t.suggestionReject }}</v-btn>
  </div>
</template>
