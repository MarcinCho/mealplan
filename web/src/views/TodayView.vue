<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useDisplay } from 'vuetify'
import type { MealNutrition, PlanEntry, Slot } from '../api/types'
import EmptyState from '../components/EmptyState.vue'
import DaySummaryCard from '../components/DaySummary.vue'
import EntryDialog from '../components/EntryDialog.vue'
import FoodLogDialog from '../components/FoodLogDialog.vue'
import MealNutritionDialog from '../components/MealNutritionDialog.vue'
import QuickAdd from '../components/QuickAdd.vue'
import { errorText, t } from '../messages'
import { useMembers } from '../stores/members'
import { usePantry } from '../stores/pantry'
import { reviewDue } from '../pantry'
import { entryChip, entryTitle, staleSessions } from '../mealprep'
import { useMealPrepStore } from '../stores/mealprep'
import { useNutrition, type LogInput } from '../stores/nutrition'
import { usePlan, type EntryInput } from '../stores/plan'
import { useRecipes } from '../stores/recipes'
import { useSession } from '../stores/session'
import { avatarColor, entriesOfDay, freeSlots, greeting, heroOf, initial, SLOT_ACCUSATIVE, SLOT_ICON } from '../today'
import { formatNumber } from '../nutrition'
import { isoDate, mondayOf, SLOTS, weekDates } from '../weeks'

const { mdAndUp } = useDisplay()
const plan = usePlan()
const recipes = useRecipes()
const members = useMembers()
const session = useSession()
const nutrition = useNutrition()
const pantry = usePantry()
const prep = useMealPrepStore()
const staleCooking = computed(() => prep.loaded && staleSessions(prep.sessions, todayDate).length > 0)
const needsReview = computed(() => pantry.loaded && reviewDue(pantry.reviewedAt, pantry.items.length, now))

const now = new Date()
const todayDate = isoDate(now)
const week = mondayOf(now)
const dates = weekDates(week)
const selected = ref(todayDate)
const error = ref('')
const ready = ref(false)
const dialog = ref(false)
const editing = ref<PlanEntry | null>(null)
const logDialog = ref(false)
const mealDialog = ref(false)
const shownMeal = ref<MealNutrition | null>(null)
const initial_ = ref<{ date: string; slot: Slot }>({ date: todayDate, slot: 'lunch' })

const recipeTitle = computed(() => Object.fromEntries(recipes.items.map((r) => [r.id, r.title])))
const ready_ = computed(() => ready.value && plan.loaded && plan.week === week)
const dayEntries = computed(() => entriesOfDay(plan.entries, selected.value))
const tracking = computed(() => nutrition.settings?.enabled === true)
const daySummary = computed(() => nutrition.days[selected.value] ?? null)
const mealOf = (e: PlanEntry) => daySummary.value?.meals.find((m) => m.entryId === e.id) ?? null
const chipText = (e: PlanEntry) => {
  const k = mealOf(e)?.values.kcal
  return k === null || k === undefined ? '—' : `${formatNumber(k, 'kcal')} kcal`
}
function openMeal(e: PlanEntry) {
  shownMeal.value = mealOf(e)
  if (shownMeal.value) mealDialog.value = true
}
async function refreshDays() {
  if (tracking.value) await nutrition.loadDays(dates[0], dates[6])
}
const hero = computed(() => heroOf(plan.entries, selected.value))
const free = computed(() => freeSlots(plan.entries, selected.value))
const isToday = computed(() => selected.value === todayDate)
const slotTitle = (s: Slot) => SLOTS.find((x) => x.value === s)?.title ?? ''
const dayTitle = computed(() => {
  const s = new Date(selected.value + 'T00:00').toLocaleDateString('pl-PL', { weekday: 'long' })
  return s.charAt(0).toUpperCase() + s.slice(1)
})
const dateLine = (() => {
  const s = now.toLocaleDateString('pl-PL', { weekday: 'long', day: 'numeric', month: 'long' })
  return s.charAt(0).toUpperCase() + s.slice(1)
})()

const cookCount = computed(() => {
  const n: Record<string, number> = {}
  for (const e of plan.entries) if (e.cookUserId) n[e.cookUserId] = (n[e.cookUserId] ?? 0) + 1
  return members.items.map((m) => ({ ...m, n: n[m.id] ?? 0 })).filter((m) => m.n > 0)
})

function label(e: PlanEntry): string {
  return entryTitle(e, e.recipeId ? (recipeTitle.value[e.recipeId] ?? '…') : null)
}
const color = (id: string) => avatarColor(id, members.ids)

async function init() {
  error.value = ''
  try {
    await Promise.all([recipes.load(), members.load()])
    ready.value = true
    await plan.load(week)
    try {
      await nutrition.loadSettings()
    } catch {
      /* nutrition stays hidden; the day loads regardless */
    }
    await refreshDays()
  } catch (e) {
    error.value = errorText(e)
  }
}
onMounted(() => {
  void init()
  pantry.load().catch(() => {}) // the reminder is optional; stay silent on failure
  prep.load().catch(() => {}) // so is this one
})

async function run(fn: () => Promise<unknown>) {
  error.value = ''
  try {
    await fn()
    await refreshDays()
  } catch (e) {
    error.value = errorText(e)
  }
}

const toggleDone = (e: PlanEntry) => run(() => plan.setDone(e, !e.done))
const quickAdd = (input: EntryInput) => run(() => plan.add(input))

function openNew(date: string, slot: Slot) {
  editing.value = null
  initial_.value = { date, slot }
  dialog.value = true
}
function openEdit(e: PlanEntry) {
  editing.value = e
  dialog.value = true
}
async function save(input: EntryInput) {
  await run(async () => {
    if (editing.value) await plan.edit(editing.value.id, input)
    else await plan.add(input)
    dialog.value = false
  })
}
// Failures of the add itself go to the dialog; a failed refresh afterwards is a page-level error.
async function submitLog(input: LogInput) {
  await nutrition.addLog(input)
  void run(async () => {}) // run() refreshes the summaries and reports a failure on the page
}
const removeLog = (id: string) => run(() => nutrition.removeLog(id))

async function removeEntry() {
  const e = editing.value
  if (!e) return
  await run(async () => {
    await plan.remove(e.id)
    dialog.value = false
  })
}
</script>

<template>
  <div class="today-layout">
    <div class="today-main">
      <div class="mb-4">
        <div class="text-body-2 text-medium-emphasis">{{ dateLine }}</div>
        <h1 class="text-h5 font-weight-bold">{{ greeting(now.getHours()) }}, {{ session.me?.user.displayName }}</h1>
      </div>

      <v-alert v-if="error" type="error" variant="tonal" class="mb-4">
        {{ error }}
        <template v-if="!ready_" #append><v-btn variant="text" @click="init">{{ t.retry }}</v-btn></template>
      </v-alert>
      <v-progress-linear v-if="!ready_ && !error" indeterminate class="mb-4" />
      <div v-if="needsReview || staleCooking" class="d-flex flex-wrap ga-2 mb-4">
      <v-chip v-if="needsReview" :to="{ name: 'pantry' }" color="warning" variant="tonal" prepend-icon="mdi-fridge-outline">{{ t.pantryReviewDue }}</v-chip>

      <v-chip v-if="staleCooking" :to="{ name: 'prep' }" color="warning" variant="tonal" prepend-icon="mdi-pot-steam-outline" style="height: auto; white-space: normal">{{ t.prepStaleToday }}</v-chip>
      </div>

      <template v-if="ready_">
        <section class="hero mb-5">
          <template v-if="hero.kind === 'next'">
            <div class="hero-tag">{{ isToday ? t.todayOn : `${dayTitle} · ${t.dayOn}` }} {{ SLOT_ACCUSATIVE[hero.entry.slot] }}</div>
            <h2 class="hero-title">{{ label(hero.entry) }}</h2>
            <div class="d-flex flex-wrap ga-2 mb-5">
              <span v-if="hero.entry.cookUserId" class="hero-chip">
                <span class="avatar avatar-sm" :style="{ background: color(hero.entry.cookUserId) }">{{ initial(members.name(hero.entry.cookUserId)) }}</span>
                {{ t.cooks }} {{ members.name(hero.entry.cookUserId) }}
              </span>
              <span class="hero-chip">{{ hero.entry.servings }} {{ t.servingsShort }}</span>
              <span v-if="entryChip(hero.entry)" class="hero-chip">{{ entryChip(hero.entry) }}</span>
            </div>
            <div class="d-flex flex-wrap ga-2">
              <v-btn class="hero-btn" size="large" prepend-icon="mdi-check" @click="toggleDone(hero.entry)">{{ t.markDone }}</v-btn>
              <v-btn
                v-if="hero.entry.recipeId"
                variant="outlined"
                size="large"
                class="hero-ghost"
                :to="{ name: 'recipe-edit', params: { id: hero.entry.recipeId } }"
              >{{ t.openRecipe }}</v-btn>
            </div>
          </template>
          <template v-else>
            <div class="hero-tag">{{ isToday ? t.today : dayTitle }}</div>
            <h2 class="hero-title">{{ hero.kind === 'finished' ? t.allCooked : t.nothingPlanned }}</h2>
            <div v-if="hero.kind === 'empty'" class="text-body-2" style="opacity: 0.8">{{ t.nothingPlannedHint }}</div>
          </template>
        </section>

        <div class="strip mb-4">
          <button
            v-for="(d, i) in dates"
            :key="d"
            class="day"
            :class="{ on: d === selected, today: d === todayDate }"
            :aria-pressed="d === selected"
            @click="selected = d"
          >
            <span>{{ t.dayNames[i] }}</span>
            <b>{{ Number(d.slice(8)) }}</b>
            <i :class="{ off: !plan.entries.some((e) => e.date === d) }" />
          </button>
        </div>

        <div class="section-label">{{ dayTitle }}</div>
        <v-card v-for="e in dayEntries" :key="e.id" class="meal mb-2" :class="{ done: e.done }" @click="openEdit(e)">
          <div class="d-flex align-center ga-3 pa-3">
            <span class="tile"><v-icon :icon="SLOT_ICON[e.slot]" /></span>
            <div class="flex-grow-1" style="min-width: 0">
              <div class="font-weight-bold meal-title">{{ label(e) }}</div>
              <div class="text-body-2 text-medium-emphasis">{{ slotTitle(e.slot) }} · {{ e.servings }} {{ t.servingsShort }}<template v-if="entryChip(e)"> · {{ entryChip(e) }}</template></div>
            </div>
            <v-chip v-if="tracking && mealOf(e)" size="small" variant="tonal" :title="t.mealNutritionTitle" @click.stop="openMeal(e)">{{ chipText(e) }}</v-chip>
            <span v-if="e.cookUserId" class="avatar" :style="{ background: color(e.cookUserId) }" :title="members.name(e.cookUserId)">
              {{ initial(members.name(e.cookUserId)) }}
            </span>
            <v-btn
              :icon="e.done ? 'mdi-check-circle' : 'mdi-circle-outline'"
              variant="text"
              :color="e.done ? 'success' : undefined"
              :title="e.done ? t.markUndone : t.markDone"
              @click.stop="toggleDone(e)"
            />
          </div>
        </v-card>
        <button v-for="s in free" :key="s" class="free" @click="openNew(selected, s)">
          <span class="tile free-tile"><v-icon icon="mdi-plus" /></span>
          <span class="text-start">
            <b>{{ t.addMeal }}: {{ slotTitle(s).toLowerCase() }}</b>
            <span class="d-block text-body-2">{{ t.whoCooks }}</span>
          </span>
        </button>

        <DaySummaryCard v-if="tracking && daySummary" class="mt-4" :day="daySummary" @add="logDialog = true" @remove="removeLog" />

        <EmptyState v-if="recipes.items.length === 0 && dayEntries.length === 0" :title="t.noRecipes" :hint="t.noRecipesHint">
          <v-btn color="primary" :to="{ name: 'recipe-new' }">{{ t.newRecipe }}</v-btn>
        </EmptyState>

        <div class="quick-dock mt-5">
          <QuickAdd :recipes="recipes.items" :members="members.items" :dates="dates" :date="selected" @add="quickAdd" />
        </div>
      </template>
    </div>

    <aside v-if="mdAndUp && ready_" class="today-side">
      <v-card class="pa-5 mb-4">
        <h3 class="text-subtitle-1 font-weight-bold mb-3">{{ t.thisWeekTitle }}</h3>
        <div class="mini">
          <div v-for="(d, i) in dates" :key="d" :class="{ f: plan.entries.some((e) => e.date === d), t: d === todayDate }">
            {{ t.dayNames[i] }}<i />
          </div>
        </div>
      </v-card>
      <v-card class="pa-5 mb-4">
        <h3 class="text-subtitle-1 font-weight-bold mb-3">{{ t.cooksThisWeek }}</h3>
        <div v-if="!cookCount.length" class="text-body-2 text-medium-emphasis">—</div>
        <div v-for="m in cookCount" :key="m.id" class="d-flex align-center ga-3 py-1">
          <span class="avatar avatar-sm" :style="{ background: color(m.id) }">{{ initial(m.displayName) }}</span>
          <span class="flex-grow-1">{{ m.displayName }}</span>
          <b>{{ m.n }}</b>
        </div>
      </v-card>
      <v-btn block variant="tonal" color="primary" size="large" :to="{ name: 'shopping' }" prepend-icon="mdi-cart">{{ t.openShopping }}</v-btn>
    </aside>
  </div>

  <EntryDialog
    v-model="dialog"
    :dates="dates"
    :recipes="recipes.items"
    :members="members.items"
    :entry="editing"
    :initial="initial_"
    @save="save"
    @remove="removeEntry"
  />
  <FoodLogDialog v-model="logDialog" :date="selected" :submit="submitLog" />
  <MealNutritionDialog v-model="mealDialog" :meal="shownMeal" />
</template>

<style scoped>
.today-layout { display: grid; grid-template-columns: minmax(0, 1fr); gap: 32px; }
@media (min-width: 960px) { .today-layout { grid-template-columns: minmax(0, 640px) 340px; justify-content: center; } }
.today-main { min-width: 0; padding-bottom: 80px; }
.hero {
  position: relative; overflow: hidden; border-radius: 32px; padding: 22px;
  background: rgb(var(--v-theme-bar)); color: rgb(var(--v-theme-on-bar));
}
.hero::after {
  content: ''; position: absolute; right: -50px; top: -50px; width: 190px; height: 190px; border-radius: 50%;
  background: rgb(var(--v-theme-primary)); opacity: 0.3; pointer-events: none;
}
.hero > * { position: relative; z-index: 1; }
.hero-tag { font-size: 12px; letter-spacing: 1px; text-transform: uppercase; opacity: 0.75; font-weight: 600; }
.hero-title { font-size: 1.75rem; line-height: 1.15; letter-spacing: -0.5px; margin: 8px 0 14px; max-width: 90%; overflow-wrap: anywhere; }
.hero-chip {
  display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px; border-radius: 12px; font-size: 13px;
  background: rgba(var(--v-theme-on-bar), 0.14);
}
.hero-btn { background: rgb(var(--v-theme-on-bar)) !important; color: rgb(var(--v-theme-bar)) !important; }
.hero-ghost { border-color: rgba(var(--v-theme-on-bar), 0.45) !important; color: rgb(var(--v-theme-on-bar)) !important; }
.avatar {
  width: 32px; height: 32px; border-radius: 50%; display: inline-grid; place-items: center; flex: none;
  font-weight: 700; font-size: 13px; color: #fff;
}
.avatar-sm { width: 22px; height: 22px; font-size: 11px; }
.strip { display: flex; gap: 8px; overflow-x: auto; scrollbar-width: none; padding: 2px; margin: 0 -2px; }
.day {
  flex: 1 0 52px; display: flex; flex-direction: column; align-items: center; gap: 2px; padding: 10px 0 12px;
  border-radius: 20px; background: rgb(var(--v-theme-surface)); border: 1px solid rgba(var(--v-theme-on-surface), 0.12);
  font: inherit; color: inherit; cursor: pointer;
}
.day span { font-size: 12px; opacity: 0.7; }
.day b { font-size: 18px; }
.day i { width: 6px; height: 6px; border-radius: 50%; background: rgb(var(--v-theme-primary)); }
.day i.off { background: transparent; }
.day.today:not(.on) { border-color: rgb(var(--v-theme-primary)); }
.day.on { background: rgb(var(--v-theme-primary)); color: rgb(var(--v-theme-on-primary)); border-color: rgb(var(--v-theme-primary)); }
.day.on i { background: rgb(var(--v-theme-on-primary)); }
.section-label { font-size: 13px; font-weight: 700; letter-spacing: 0.8px; text-transform: uppercase; opacity: 0.65; margin: 8px 4px 10px; }
.meal { cursor: pointer; }
.meal.done .meal-title { text-decoration: line-through; opacity: 0.6; }
.tile {
  width: 44px; height: 44px; border-radius: 14px; display: grid; place-items: center; flex: none;
  background: rgb(var(--v-theme-surface-variant)); color: rgb(var(--v-theme-on-surface-variant, var(--v-theme-on-surface)));
}
.free {
  width: 100%; display: flex; align-items: center; gap: 12px; padding: 12px; margin-bottom: 8px; border-radius: 24px;
  border: 1.5px dashed rgba(var(--v-theme-on-surface), 0.3); background: transparent; color: inherit; font: inherit; cursor: pointer; opacity: 0.8;
}
.free-tile { background: transparent; border: 1.5px dashed rgba(var(--v-theme-on-surface), 0.3); }
.quick-dock { position: sticky; bottom: calc(76px + env(safe-area-inset-bottom)); z-index: 3; }
@media (min-width: 960px) { .quick-dock { bottom: 24px; } }
.mini { display: grid; grid-template-columns: repeat(7, 1fr); gap: 6px; text-align: center; font-size: 12px; opacity: 0.9; }
.mini i { display: block; height: 44px; border-radius: 12px; margin-top: 4px; background: rgba(var(--v-theme-on-surface), 0.07); }
.mini .f i { background: rgb(var(--v-theme-surface-variant)); }
.mini .t i { background: rgb(var(--v-theme-primary)); }
</style>
