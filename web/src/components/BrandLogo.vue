<script setup lang="ts">
import { computed } from 'vue'
import { useTheme } from 'vuetify'
import { logoVariantFor } from '../brand'
import lightWordmark from '../assets/brand/mealplan-light.svg'
import darkWordmark from '../assets/brand/mealplan-dark.svg'
import lightIcon from '../assets/brand/mealplan-light-favi.svg'
import darkIcon from '../assets/brand/mealplan-dark-favi.svg'

// `background` is the hex the logo sits on (default: the page background); the variant with readable text is picked from it.
const props = withDefaults(defineProps<{ background?: string; icon?: boolean; height?: number }>(), { background: undefined, icon: false, height: 28 })
const theme = useTheme()

const src = computed(() => {
  const dark = logoVariantFor(props.background ?? (theme.current.value.colors.background as string)) === 'dark'
  if (props.icon) return dark ? darkIcon : lightIcon
  return dark ? darkWordmark : lightWordmark
})
</script>

<template>
  <img :src="src" alt="Mealplan" :height="height" :width="icon ? height : undefined" class="brand-logo" />
</template>

<style scoped>
.brand-logo { display: block; width: auto; max-width: 100%; }
</style>
