import { createRouter, createWebHistory } from 'vue-router'
import { resolveRoute } from './guard'
import { useSession } from './stores/session'
import { isStaleChunk } from './chunks'
import AppShell from './layouts/AppShell.vue'
import LoginView from './views/LoginView.vue'
import TodayView from './views/TodayView.vue'

// Rarely the first screen, so loaded on demand; Today and Login stay in the main bundle.
const AccountView = () => import('./views/AccountView.vue')
const ImportView = () => import('./views/ImportView.vue')
const InviteAcceptView = () => import('./views/InviteAcceptView.vue')
const MembersView = () => import('./views/MembersView.vue')
const MealPrepView = () => import('./views/MealPrepView.vue')
const PantryView = () => import('./views/PantryView.vue')
const PlanView = () => import('./views/PlanView.vue')
const ProductCheckView = () => import('./views/ProductCheckView.vue')
const ProductsView = () => import('./views/ProductsView.vue')
const RecipeEditView = () => import('./views/RecipeEditView.vue')
const RecipeCheckView = () => import('./views/RecipeCheckView.vue')
const RecipesView = () => import('./views/RecipesView.vue')
const SetupView = () => import('./views/SetupView.vue')
const ShoppingView = () => import('./views/ShoppingView.vue')

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/setup', name: 'setup', component: SetupView, meta: { public: true } },
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    { path: '/invite/:token', name: 'invite', component: InviteAcceptView, meta: { public: true } },
    {
      path: '/',
      component: AppShell,
      children: [
        { path: '', name: 'home', component: TodayView },
        { path: 'plan', name: 'plan', component: PlanView },
        { path: 'prep', name: 'prep', component: MealPrepView },
        { path: 'recipes', name: 'recipes', component: RecipesView },
        { path: 'recipes/import', name: 'recipe-import', component: ImportView },
        { path: 'recipes/check', name: 'recipe-check', component: RecipeCheckView },
        { path: 'recipes/new', name: 'recipe-new', component: RecipeEditView },
        { path: 'recipes/:id', name: 'recipe-edit', component: RecipeEditView },
        { path: 'shopping', name: 'shopping', component: ShoppingView },
        { path: 'pantry', name: 'pantry', component: PantryView },
        { path: 'account', name: 'account', component: AccountView },
        { path: 'products', name: 'products', component: ProductsView },
        { path: 'products/check', name: 'product-check', component: ProductCheckView },
        { path: 'members', name: 'members', component: MembersView },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  const session = useSession()
  if (!session.loaded) await session.bootstrap()
  return resolveRoute(
    { me: session.me, setupRequired: session.setupRequired },
    to.name as string | undefined,
    Boolean(to.meta.public),
  )
})

// A tab opened before a deploy still points at the old view chunks, which are gone now: load the
// page again (once) so the browser picks up the new build.
router.onError((err, to) => {
  if (!isStaleChunk(err)) return
  try {
    if (sessionStorage.getItem('mealplan.reloaded') === to.fullPath) return
    sessionStorage.setItem('mealplan.reloaded', to.fullPath)
  } catch {
    // storage blocked: reload anyway
  }
  window.location.assign(to.fullPath)
})

router.afterEach(() => {
  try {
    sessionStorage.removeItem('mealplan.reloaded')
  } catch {
    // ignore
  }
})
