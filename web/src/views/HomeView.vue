<script setup lang="ts">
import { useSession } from '../stores/session'
import { t } from '../messages'

const session = useSession()
</script>

<template>
  <h1 class="text-h5 mb-2">{{ session.me?.household.name }}</h1>
  <p class="text-medium-emphasis">{{ t.homeSoon }}</p>
</template>
