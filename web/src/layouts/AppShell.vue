<script setup lang="ts">
import { useDisplay, useTheme } from 'vuetify'
import { useRoute, useRouter } from 'vue-router'
import { useSession } from '../stores/session'
import { useNotice } from '../stores/notice'
import { t } from '../messages'
import { loadChoice, PALETTES, saveChoice, themeName, type Choice, type PaletteId } from '../theme'
import { initial } from '../today'
import { computed, ref, watch } from 'vue'
import BrandLogo from '../components/BrandLogo.vue'

const { mdAndUp, lgAndUp } = useDisplay()
const theme = useTheme()
const session = useSession()
const notice = useNotice()
const router = useRouter()
const route = useRoute()
// The account menu stays open while picking colours, but closes once one of its links opens a page.
const menuOpen = ref(false)
watch(() => route.fullPath, () => (menuOpen.value = false))

const items = [
  { title: t.navToday, icon: 'mdi-home-variant-outline', active: 'mdi-home-variant', to: { name: 'home' } },
  { title: t.navPlan, icon: 'mdi-calendar-week-outline', active: 'mdi-calendar-week', to: { name: 'plan' } },
  { title: t.navPrep, icon: 'mdi-pot-steam-outline', active: 'mdi-pot-steam', to: { name: 'prep' } },
  { title: t.navRecipes, icon: 'mdi-book-open-variant-outline', active: 'mdi-book-open-variant', to: { name: 'recipes' } },
  { title: t.navPantry, icon: 'mdi-fridge-outline', active: 'mdi-fridge', to: { name: 'pantry' } },
  { title: t.navShopping, icon: 'mdi-cart-outline', active: 'mdi-cart', to: { name: 'shopping' } },
]

// The full name goes in the title attribute; the bar only truncates it visually.
const householdName = computed(() => session.me?.household.name ?? '')

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
  <v-app-bar color="background" flat class="shell-bar">
    <div class="bar-inner">
      <BrandLogo :icon="!mdAndUp" :height="28" class="brand" />
      <nav v-if="mdAndUp" :aria-label="t.navMain" class="top-nav">
        <router-link v-for="i in items" :key="i.title" v-slot="{ href, navigate, isActive, isExactActive }" :to="i.to" custom>
          <a
            :href="href"
            class="nav-link"
            :class="{ on: i.to.name === 'home' ? isExactActive : isActive }"
            :aria-current="(i.to.name === 'home' ? isExactActive : isActive) ? 'page' : undefined"
            @click="navigate"
          >{{ i.title }}</a>
        </router-link>
      </nav>
      <v-spacer />
      <span v-if="lgAndUp" class="household-name text-body-2 text-medium-emphasis mr-3" :title="householdName">{{ householdName }}</span>
      <v-menu v-model="menuOpen" :close-on-content-click="false">
        <template #activator="{ props }">
          <v-btn v-bind="props" icon variant="text" :title="t.account">
            <span class="avatar-btn">{{ initial(session.me?.user.displayName ?? '') }}</span>
          </v-btn>
        </template>
        <v-list min-width="240" class="pa-2">
          <v-list-subheader v-if="!lgAndUp" class="household-name" :title="householdName">{{ householdName }}</v-list-subheader>
          <v-list-subheader>{{ t.palette }}</v-list-subheader>
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
          <v-divider class="my-2" />
          <v-list-item prepend-icon="mdi-theme-light-dark" :title="t.theme" @click="toggleTheme" />
          <v-list-item prepend-icon="mdi-account-cog-outline" :title="t.accountTitle" :to="{ name: 'account' }" />
          <v-list-item prepend-icon="mdi-food-apple-outline" :title="t.navProducts" :to="{ name: 'products' }" />
          <v-list-item prepend-icon="mdi-home-edit-outline" :title="t.householdSettings" :to="{ name: 'household' }" />
          <v-list-item prepend-icon="mdi-account-group-outline" :title="t.navMembers" :to="{ name: 'members' }" />
          <v-list-item prepend-icon="mdi-logout" :title="t.signOut" @click="logout" />
        </v-list>
      </v-menu>
    </div>
  </v-app-bar>

  <v-main>
    <v-container class="py-4 content">
      <router-view v-slot="{ Component }">
        <transition name="page" mode="out-in"><component :is="Component" /></transition>
      </router-view>
    </v-container>
  </v-main>

  <v-bottom-navigation v-if="!mdAndUp" tag="nav" :aria-label="t.navMain" grow class="bottom-nav" height="72">
    <router-link v-for="i in items" :key="i.title" v-slot="{ href, navigate, isActive, isExactActive }" :to="i.to" custom>
      <a
        :href="href"
        class="nav-item"
        :class="{ on: i.to.name === 'home' ? isExactActive : isActive }"
        :aria-current="(i.to.name === 'home' ? isExactActive : isActive) ? 'page' : undefined"
        @click="navigate"
      >
        <v-icon :icon="(i.to.name === 'home' ? isExactActive : isActive) ? i.active : i.icon" />
        {{ i.title }}
      </a>
    </router-link>
  </v-bottom-navigation>

  <v-snackbar
    :key="notice.seq"
    v-model="notice.open"
    :timeout="4000"
    color="secondary"
    :location="mdAndUp ? 'bottom' : 'top'"
    :class="{ 'notice-phone': !mdAndUp }"
    role="status"
  >
    <span class="d-inline-flex align-center ga-2"><v-icon icon="mdi-check-circle" size="small" />{{ notice.text }}</span>
    <template #actions>
      <v-btn variant="text" :aria-label="t.close" icon="mdi-close" size="small" @click="notice.open = false" />
    </template>
  </v-snackbar>
</template>

<style scoped>
.swatch { width: 14px; height: 14px; border-radius: 50%; border: 1px solid rgba(128, 128, 128, 0.45); }
.avatar-btn {
  width: 36px; height: 36px; border-radius: 50%; display: grid; place-items: center; font-weight: 700;
  background: rgb(var(--v-theme-primary)); color: rgb(var(--v-theme-on-primary));
}
.shell-bar { border-bottom: 1px solid var(--mp-line); }
.bar-inner { display: flex; align-items: center; gap: 24px; width: 100%; max-width: var(--mp-content-max); margin: 0 auto; padding: 0 16px; }
.brand { flex: none; }
.top-nav { display: flex; align-items: center; gap: 4px; }
.nav-link {
  padding: 8px 14px; border-radius: var(--mp-radius-control); font-size: 16px; font-weight: 500;
  text-decoration: none; color: rgb(var(--v-theme-on-surface));
}
.nav-link:hover { background: rgba(var(--v-theme-on-surface), 0.06); }
.nav-link.on { background: rgb(var(--v-theme-surface-variant)); font-weight: 700; }
.household-name { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.content { max-width: var(--mp-content-max); margin-inline: auto; }
.bottom-nav { background: rgb(var(--v-theme-surface)) !important; border-top: 1px solid var(--mp-line); }
.nav-item {
  display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 4px; flex: 1; min-width: 0;
  margin: 8px 2px; border-radius: var(--mp-radius-control); font-size: 11px; font-weight: 500;
  text-decoration: none; color: rgba(var(--v-theme-on-surface), 0.7);
}
.nav-item.on { background: rgb(var(--v-theme-surface-variant)); color: rgb(var(--v-theme-on-surface)); font-weight: 700; }
.page-enter-active, .page-leave-active { transition: opacity 0.15s ease, transform 0.15s ease; }
.page-enter-from { opacity: 0; transform: translateY(6px); }
.page-leave-to { opacity: 0; }
@media (prefers-reduced-motion: reduce) { .page-enter-active, .page-leave-active { transition: none; } }
</style>

<style>
/* On phones the notice sits below the app bar, clear of the bottom navigation and quick add. */
.notice-phone .v-snackbar__wrapper { margin-top: 72px; }
</style>
