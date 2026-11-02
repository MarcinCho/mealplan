import { createRouter, createWebHistory } from 'vue-router'
import { resolveRoute } from './guard'
import { useSession } from './stores/session'
import AppShell from './layouts/AppShell.vue'
import InviteAcceptView from './views/InviteAcceptView.vue'
import LoginView from './views/LoginView.vue'
import MembersView from './views/MembersView.vue'
import PlanView from './views/PlanView.vue'
import RecipeEditView from './views/RecipeEditView.vue'
import RecipesView from './views/RecipesView.vue'
import SetupView from './views/SetupView.vue'
import ShoppingView from './views/ShoppingView.vue'
import TodayView from './views/TodayView.vue'

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
        { path: 'recipes', name: 'recipes', component: RecipesView },
        { path: 'recipes/new', name: 'recipe-new', component: RecipeEditView },
        { path: 'recipes/:id', name: 'recipe-edit', component: RecipeEditView },
        { path: 'shopping', name: 'shopping', component: ShoppingView },
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
