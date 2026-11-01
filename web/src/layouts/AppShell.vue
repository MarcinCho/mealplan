<script setup lang="ts">
import { useDisplay, useTheme } from 'vuetify'
import { useRouter } from 'vue-router'
import { useSession } from '../stores/session'
import { t } from '../messages'

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

function toggleTheme() {
  theme.global.name.value = theme.global.current.value.dark ? 'light' : 'dark'
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
  <v-app-bar flat border="b">
    <v-app-bar-title>{{ t.appName }}</v-app-bar-title>
    <span class="text-body-2 mr-2 d-none d-sm-inline">{{ session.me?.household.name }}</span>
    <v-btn icon variant="text" :title="t.theme" @click="toggleTheme"><v-icon>mdi-theme-light-dark</v-icon></v-btn>
    <v-btn icon variant="text" :title="t.signOut" @click="logout"><v-icon>mdi-logout</v-icon></v-btn>
  </v-app-bar>

  <v-navigation-drawer v-if="mdAndUp" rail permanent>
    <v-list nav>
      <v-list-item v-for="i in items" :key="i.title" :to="i.to" :prepend-icon="i.icon" :title="i.title" />
    </v-list>
  </v-navigation-drawer>

  <v-main>
    <v-container class="py-6" style="max-width: 960px"><router-view /></v-container>
  </v-main>

  <v-bottom-navigation v-if="!mdAndUp" grow>
    <v-btn v-for="i in items" :key="i.title" :to="i.to"><v-icon>{{ i.icon }}</v-icon>{{ i.title }}</v-btn>
  </v-bottom-navigation>
</template>
