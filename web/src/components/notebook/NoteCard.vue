<script setup lang="ts">
withDefaults(defineProps<{ label?: string; title?: string; titleTag?: 'h1' | 'h2' | 'h3' }>(), { titleTag: 'h2' })
</script>

<template>
  <section class="note-card">
    <header class="note-card__header bg-note">
      <div v-if="label || $slots.label" class="note-card__label"><slot name="label">{{ label }}</slot></div>
      <component :is="titleTag" v-if="title || $slots.title" class="note-card__title mp-display"><slot name="title">{{ title }}</slot></component>
      <div v-if="$slots.actions" class="note-card__actions"><slot name="actions" /></div>
    </header>
    <div v-if="$slots.subheader" class="note-card__subheader"><slot name="subheader" /></div>
    <div v-if="$slots.default" class="note-card__body mp-grid"><slot /></div>
  </section>
</template>

<style scoped>
.note-card {
  border-radius: var(--mp-radius-card);
  overflow: hidden;
  background: rgb(var(--v-theme-surface));
  box-shadow: 0 1px 1px rgb(35 56 49 / 0.1), 0 2px 4px -1px rgb(35 56 49 / 0.1), 0 24px 44px -20px rgb(35 56 49 / 0.42);
}
.note-card__header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 12px;
  padding: 12px 16px;
}
.note-card__label {
  flex: 0 0 100%;
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  opacity: 0.8;
}
.note-card__title {
  flex: 1 1 0;
  min-width: 0;
  margin: 0;
  font-size: 1.5rem;
  line-height: 1.2;
  overflow-wrap: anywhere;
}
.note-card__actions { display: flex; align-items: center; gap: 8px; margin-left: auto; }
.note-card__subheader { padding: 8px 16px; border-bottom: 1px solid var(--mp-line); }
.note-card__body { padding: 16px; }
</style>
