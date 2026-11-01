<script setup lang="ts">
import { useDisplay, useTheme } from 'vuetify'
import { useRouter } from 'vue-router'
import { useSession } from '../stores/session'
import { t } from '../messages'
import { loadChoice, PALETTES, saveChoice, themeName, type Choice, type PaletteId } from '../theme'
import { ref } from 'vue'

const { mdAndUp } = useDisplay()
const theme = useTheme()
const session = useSession()
const router = useRouter()

const items = [
  { title: t.navPlan, icon: 'mdi-calendar-week', to: { name: 'home' } },
  { title: t.navRecipes, icon: 'mdi-book-open-variant', to: { name: 'recipes' } },
  { title: t.navShopping, icon: 'mdi-cart', to: { name: 'shopping' } },
  { title: t.navMembers, icon: 'mdi-account-group', to: { name: 'members' } },
]

const choice = ref<Choice>(loadChoice())

function applyChoice(c: Choice) {
  choice.value = c
  theme.global.name.value = themeName(c)
  saveChoice(c)
}

function toggleTheme() {
  applyChoice({ ...choice.value, mode: choice.value.mode === 'dark' ? 'light' : 'dark' })
}

function pickPalette(palette: PaletteId) {
  applyChoice({ ...choice.value, palette })
}

async function logout() {
  try {
    await session.logout()
  } catch {
    // local state is cleared by the store; the login page is the sane place to land
  } finally {
    await router.push({ name: 'login' })
  }
}
</script>

<template>
  <v-app-bar flat border="b" color="bar">
    <v-app-bar-title>{{ t.appName }}</v-app-bar-title>
    <span class="text-body-2 mr-2 d-none d-sm-inline">{{ session.me?.household.name }}</span>
    <v-menu>
      <template #activator="{ props }">
        <v-btn v-bind="props" icon variant="text" :title="t.palette"><v-icon>mdi-palette</v-icon></v-btn>
      </template>
      <v-list density="compact" min-width="200">
        <v-list-item
          v-for="p in PALETTES"
          :key="p.id"
          :active="p.id === choice.palette"
          :title="t.paletteNames[p.id]"
          @click="pickPalette(p.id)"
        >
          <template #append>
            <span class="d-flex ga-1">
              <span v-for="c in p.swatch" :key="c" class="swatch" :style="{ backgroundColor: c }" />
            </span>
          </template>
        </v-list-item>
      </v-list>
    </v-menu>
    <v-btn icon variant="text" :title="t.theme" @click="toggleTheme"><v-icon>mdi-theme-light-dark</v-icon></v-btn>
    <v-btn icon variant="text" :title="t.signOut" @click="logout"><v-icon>mdi-logout</v-icon></v-btn>
  </v-app-bar>

  <v-navigation-drawer v-if="mdAndUp" rail permanent>
    <v-list nav color="primary">
      <v-list-item v-for="i in items" :key="i.title" :to="i.to" :prepend-icon="i.icon" :title="i.title" />
    </v-list>
  </v-navigation-drawer>

  <v-main>
    <v-container class="py-6" style="max-width: 1200px"><router-view /></v-container>
  </v-main>

  <v-bottom-navigation v-if="!mdAndUp" grow color="primary">
    <v-btn v-for="i in items" :key="i.title" :to="i.to"><v-icon>{{ i.icon }}</v-icon>{{ i.title }}</v-btn>
  </v-bottom-navigation>
</template>

<style scoped>
.swatch {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 1px solid rgba(128, 128, 128, 0.45);
}
</style>
