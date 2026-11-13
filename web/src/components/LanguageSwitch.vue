<script setup lang="ts">
import { LOCALES, locale, setLocale, type Locale } from '../i18n'
import { t } from '../messages'

// Switching reloads the page, so the whole interface comes back in the new language.
function choose(v: Locale | null) {
  if (v) setLocale(v)
}
</script>

<template>
  <div class="d-flex align-center flex-wrap ga-3">
    <span class="text-body-2 text-medium-emphasis" id="language-label">{{ t.language }}</span>
    <v-btn-toggle :model-value="locale" mandatory density="comfortable" variant="outlined" divided rounded="pill" aria-labelledby="language-label" @update:model-value="choose">
      <v-btn v-for="l in LOCALES" :key="l.value" :value="l.value" :lang="l.value">{{ l.title }}</v-btn>
    </v-btn-toggle>
  </div>
</template>
