<script setup lang="ts">
import { useDisplay, useTheme } from 'vuetify'
import { useRouter } from 'vue-router'
import { useSession } from '../stores/session'
import { t } from '../messages'
import { loadChoice, PALETTES, saveChoice, themeName, type Choice, type PaletteId } from '../theme'
import { initial } from '../today'
import { ref } from 'vue'
import BrandLogo from '../components/BrandLogo.vue'

const { mdAndUp } = useDisplay()
const theme = useTheme()
const session = useSession()
const router = useRouter()

const items = [
  { title: t.navToday, icon: 'mdi-home-variant-outline', active: 'mdi-home-variant', to: { name: 'home' } },
  { title: t.navPlan, icon: 'mdi-calendar-week-outline', active: 'mdi-calendar-week', to: { name: 'plan' } },
  { title: t.navPrep, icon: 'mdi-pot-steam-outline', active: 'mdi-pot-steam', to: { name: 'prep' } },
  { title: t.navRecipes, icon: 'mdi-book-open-variant-outline', active: 'mdi-book-open-variant', to: { name: 'recipes' } },
  { title: t.navPantry, icon: 'mdi-fridge-outline', active: 'mdi-fridge', to: { name: 'pantry' } },
  { title: t.navShopping, icon: 'mdi-cart-outline', active: 'mdi-cart', to: { name: 'shopping' } },
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
  <v-app-bar flat color="transparent" class="shell-bar">
    <v-app-bar-title class="brand-title"><BrandLogo :icon="!mdAndUp" :height="28" /></v-app-bar-title>
    <span class="text-body-2 text-medium-emphasis mr-2 d-none d-sm-inline">{{ session.me?.household.name }}</span>
    <v-menu :close-on-content-click="false">
      <template #activator="{ props }">
        <v-btn v-bind="props" icon variant="text" :title="t.account" class="mr-2">
          <span class="avatar-btn">{{ initial(session.me?.user.displayName ?? '') }}</span>
        </v-btn>
      </template>
      <v-list min-width="240" rounded="xl" class="pa-2">
        <v-list-subheader>{{ t.palette }}</v-list-subheader>
        <v-list-item
          v-for="p in PALETTES"
          :key="p.id"
          :active="p.id === choice.palette"
          :title="t.paletteNames[p.id]"
          rounded="lg"
          @click="pickPalette(p.id)"
        >
          <template #append>
            <span class="d-flex ga-1">
              <span v-for="c in p.swatch" :key="c" class="swatch" :style="{ backgroundColor: c }" />
            </span>
          </template>
        </v-list-item>
        <v-divider class="my-2" />
        <v-list-item prepend-icon="mdi-theme-light-dark" :title="t.theme" rounded="lg" @click="toggleTheme" />
        <v-list-item prepend-icon="mdi-account-cog-outline" :title="t.accountTitle" :to="{ name: 'account' }" rounded="lg" />
        <v-list-item prepend-icon="mdi-food-apple-outline" :title="t.navProducts" :to="{ name: 'products' }" rounded="lg" />
        <v-list-item prepend-icon="mdi-account-group-outline" :title="t.navMembers" :to="{ name: 'members' }" rounded="lg" />
        <v-list-item prepend-icon="mdi-logout" :title="t.signOut" rounded="lg" @click="logout" />
      </v-list>
    </v-menu>
  </v-app-bar>

  <v-navigation-drawer v-if="mdAndUp" permanent width="88" color="bar" class="rail">
    <nav class="rail-nav">
      <router-link v-for="i in items" :key="i.title" v-slot="{ href, navigate, isActive, isExactActive }" :to="i.to" custom>
        <a :href="href" class="nav-item" :class="{ on: i.to.name === 'home' ? isExactActive : isActive }" @click="navigate">
          <span class="pill"><v-icon :icon="(i.to.name === 'home' ? isExactActive : isActive) ? i.active : i.icon" /></span>
          {{ i.title }}
        </a>
      </router-link>
    </nav>
  </v-navigation-drawer>

  <v-main>
    <v-container class="py-4" style="max-width: 1200px">
      <router-view v-slot="{ Component }">
        <transition name="page" mode="out-in"><component :is="Component" /></transition>
      </router-view>
    </v-container>
  </v-main>

  <v-bottom-navigation v-if="!mdAndUp" grow class="bottom-nav" height="72">
    <router-link v-for="i in items" :key="i.title" v-slot="{ href, navigate, isActive, isExactActive }" :to="i.to" custom>
      <a :href="href" class="nav-item" :class="{ on: i.to.name === 'home' ? isExactActive : isActive }" @click="navigate">
        <span class="pill"><v-icon :icon="(i.to.name === 'home' ? isExactActive : isActive) ? i.active : i.icon" /></span>
        {{ i.title }}
      </a>
    </router-link>
  </v-bottom-navigation>
</template>

<style scoped>
.swatch { width: 14px; height: 14px; border-radius: 50%; border: 1px solid rgba(128, 128, 128, 0.45); }
.avatar-btn {
  width: 36px; height: 36px; border-radius: 50%; display: grid; place-items: center; font-weight: 700;
  background: rgb(var(--v-theme-primary)); color: rgb(var(--v-theme-on-primary));
}
.rail-nav { display: flex; flex-direction: column; gap: 22px; padding: 28px 0; align-items: center; }
.nav-item {
  display: flex; flex-direction: column; align-items: center; gap: 4px; font-size: 12px; font-weight: 500;
  text-decoration: none; color: rgba(var(--v-theme-on-bar), 0.7); flex: 1;
}
.nav-item .pill { width: 56px; height: 32px; border-radius: 16px; display: grid; place-items: center; transition: background 0.2s; }
.nav-item.on { color: rgb(var(--v-theme-on-bar)); font-weight: 700; }
.nav-item.on .pill { background: rgba(var(--v-theme-on-bar), 0.18); }
.bottom-nav { background: rgb(var(--v-theme-surface-variant)) !important; }
.bottom-nav .nav-item { color: rgba(var(--v-theme-on-surface), 0.7); justify-content: center; min-width: 0; font-size: 11px; }
.bottom-nav .nav-item.on { color: rgb(var(--v-theme-on-surface)); }
.bottom-nav .nav-item.on .pill { background: rgb(var(--v-theme-primary)); color: rgb(var(--v-theme-on-primary)); }
.page-enter-active, .page-leave-active { transition: opacity 0.15s ease, transform 0.15s ease; }
.page-enter-from { opacity: 0; transform: translateY(6px); }
.page-leave-to { opacity: 0; }
@media (prefers-reduced-motion: reduce) { .page-enter-active, .page-leave-active { transition: none; } }
</style>
