<script setup lang="ts">
withDefaults(defineProps<{ label?: string; highlighted?: boolean; tag?: string }>(), { highlighted: false, tag: 'div' })
</script>

<template>
  <component :is="tag" class="note-row" :class="{ 'note-row--highlighted': highlighted }">
    <div class="note-row__label"><slot name="label">{{ label }}</slot></div>
    <div class="note-row__content"><slot /></div>
    <div v-if="$slots.action" class="note-row__action"><slot name="action" /></div>
  </component>
</template>

<style scoped>
.note-row {
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--mp-line);
}
.note-row--highlighted { background: rgb(var(--v-theme-surface-variant)); }
.note-row__label {
  flex: 0 0 100px;
  align-self: stretch;
  display: flex;
  align-items: center;
  padding: 8px 12px;
  border-right: 1px solid var(--mp-line);
  font-size: 0.8125rem;
  opacity: 0.8;
}
.note-row__content { flex: 1 1 0; min-width: 0; padding: 8px 12px; overflow-wrap: anywhere; }
.note-row__action { flex: 0 0 auto; padding: 4px 8px; }
</style>
