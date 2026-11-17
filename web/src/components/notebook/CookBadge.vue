<script setup lang="ts">
import { initial } from '../../today'

withDefaults(defineProps<{ name: string; color: string; showName?: boolean }>(), { showName: true })
</script>

<template>
  <span class="cook-badge" :title="name">
    <span class="cook-badge__avatar" :style="{ backgroundColor: color }" aria-hidden="true">{{ initial(name) }}</span>
    <template v-if="showName">{{ name }}</template>
    <span v-else class="d-sr-only">{{ name }}</span>
  </span>
</template>

<style scoped>
.cook-badge { display: inline-flex; align-items: center; gap: 6px; }
.cook-badge__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  color: #fff;
  font-size: 0.75rem;
  font-weight: 600;
  flex: none;
}
</style>
