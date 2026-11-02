<script setup lang="ts">
defineProps<{ title: string; hint?: string }>()
</script>

<template>
  <div class="empty text-center py-10 px-4">
    <svg viewBox="0 0 120 90" width="120" height="90" aria-hidden="true">
      <ellipse cx="60" cy="80" rx="38" ry="6" fill="rgb(var(--v-theme-surface-variant))" opacity=".6" />
      <path d="M22 44h76a38 38 0 0 1-76 0z" fill="rgb(var(--v-theme-primary))" opacity=".9" />
      <path d="M44 30c0-8 8-8 8-16M62 30c0-8 8-8 8-16M80 30c0-8 8-8 8-16" fill="none" stroke="rgb(var(--v-theme-secondary))" stroke-width="3.5" stroke-linecap="round" />
    </svg>
    <div class="text-subtitle-1 font-weight-bold mt-3">{{ title }}</div>
    <div v-if="hint" class="text-body-2 text-medium-emphasis mt-1">{{ hint }}</div>
    <div class="mt-4"><slot /></div>
  </div>
</template>
