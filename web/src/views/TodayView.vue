<script setup lang="ts">
import { intl } from '../i18n'
import { portions } from '../polish'
import { computed, onMounted, ref } from 'vue'
import { useDisplay } from 'vuetify'
import type { MealNutrition, PlanEntry, Slot } from '../api/types'
import EmptyState from '../components/EmptyState.vue'
import DaySummaryCard from '../components/DaySummary.vue'
import EntryDialog from '../components/EntryDialog.vue'
import FoodLogDialog from '../components/FoodLogDialog.vue'
import MealNutritionDialog from '../components/MealNutritionDialog.vue'
import QuickAdd from '../components/QuickAdd.vue'
import CookBadge from '../components/notebook/CookBadge.vue'
import NoteCard from '../components/notebook/NoteCard.vue'
import NoteRow from '../components/notebook/NoteRow.vue'
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
import { avatarColor, entriesOfDay, freeSlots, greeting, heroOf, SLOT_ACCUSATIVE } from '../today'
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
  const s = new Date(selected.value + 'T00:00').toLocaleDateString(intl, { weekday: 'long' })
  return s.charAt(0).toUpperCase() + s.slice(1)
})
const dateLine = (() => {
  const s = now.toLocaleDateString(intl, { weekday: 'long', day: 'numeric', month: 'long' })
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
// An entry whose cook is no longer a household member shows no badge.
const cookName = (e: PlanEntry) => (e.cookUserId ? members.name(e.cookUserId) : '')

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
            <b class="mp-num">{{ Number(d.slice(8)) }}</b>
            <i :class="{ off: !plan.entries.some((e) => e.date === d) }" />
          </button>
        </div>

        <NoteCard v-if="hero.kind === 'next'" class="mb-5">
          <template #label>{{ isToday ? t.todayOn : `${dayTitle} · ${t.dayOn}` }} {{ SLOT_ACCUSATIVE[hero.entry.slot] }}</template>
          <template #title><span class="hero-title">{{ label(hero.entry) }}</span></template>
          <div class="hero-facts">
            <div v-if="cookName(hero.entry)" class="hero-fact">
              {{ t.cooks }} <CookBadge :name="cookName(hero.entry)" :color="color(hero.entry.cookUserId!)" />
            </div>
            <div class="hero-fact mp-num">{{ portions(hero.entry.servings) }}</div>
            <div v-if="entryChip(hero.entry)" class="hero-fact">{{ entryChip(hero.entry) }}</div>
          </div>
          <div class="hero-actions">
            <v-btn color="primary" variant="flat" size="large" prepend-icon="mdi-check" class="hero-done" @click="toggleDone(hero.entry)">{{ t.markDone }}</v-btn>
            <v-btn
              v-if="hero.entry.recipeId"
              size="large"
              :to="{ name: 'recipe', params: { id: hero.entry.recipeId }, query: { servings: String(hero.entry.servings) } }"
            >{{ t.openRecipe }}</v-btn>
          </div>
        </NoteCard>
        <NoteCard v-else class="mb-5" :label="isToday ? t.today : dayTitle">
          <template #title><span class="hero-title">{{ hero.kind === 'finished' ? t.allCooked : t.nothingPlanned }}</span></template>
          <div v-if="hero.kind === 'empty'" class="hero-facts"><div class="hero-fact">{{ t.nothingPlannedHint }}</div></div>
        </NoteCard>

        <div class="section-label">{{ dayTitle }}</div>
        <div class="day-rows">
          <NoteRow v-for="e in dayEntries" :key="e.id" :label="slotTitle(e.slot)" class="meal" :class="{ done: e.done }" @click="openEdit(e)">
            <button type="button" class="meal-title">{{ label(e) }}</button>
            <div class="meal-meta">
              <CookBadge v-if="cookName(e)" :name="cookName(e)" :color="color(e.cookUserId!)" />
              <span class="mp-num">{{ portions(e.servings) }}</span>
              <v-chip v-if="tracking && mealOf(e)" size="small" variant="tonal" class="mp-num" :title="t.mealNutritionTitle" @click.stop="openMeal(e)">{{ chipText(e) }}</v-chip>
              <span v-if="entryChip(e)">{{ entryChip(e) }}</span>
            </div>
            <template #action>
              <v-btn
                :color="e.done ? 'success' : undefined"
                :variant="e.done ? 'flat' : 'outlined'"
                :title="e.done ? t.markUndone : t.markDone"
                :aria-label="`${e.done ? t.markUndone : t.markDone}: ${label(e)}`"
                class="done-btn"
                @click.stop="toggleDone(e)"
              ><v-icon v-if="e.done" icon="mdi-check" /></v-btn>
            </template>
          </NoteRow>
          <NoteRow v-for="s in free" :key="s" :label="slotTitle(s)">
            <button type="button" class="free-btn" @click="openNew(selected, s)">
              <v-icon icon="mdi-plus" size="small" />{{ t.addMeal }}: {{ slotTitle(s).toLowerCase() }}
            </button>
            <div class="free-hint">{{ t.whoCooks }}</div>
          </NoteRow>
        </div>

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
      <h3 class="side-title">{{ t.thisWeekTitle }}</h3>
      <div class="week-rows">
        <NoteRow
          v-for="(d, i) in dates"
          :key="d"
          class="week-row"
          :highlighted="d === todayDate"
          :aria-current="d === todayDate ? 'date' : undefined"
        >
          <template #label><span>{{ t.dayNames[i] }}</span><b class="mp-num week-num">{{ Number(d.slice(8)) }}</b></template>
          <span class="week-meals">{{ entriesOfDay(plan.entries, d).map(label).join(', ') }}</span>
        </NoteRow>
      </div>

      <h3 class="side-title mt-6">{{ t.cooksThisWeek }}</h3>
      <div v-if="!cookCount.length" class="text-body-2 text-medium-emphasis py-2">—</div>
      <div v-for="m in cookCount" :key="m.id" class="cook-row mp-ruled">
        <CookBadge :name="m.displayName" :color="color(m.id)" />
        <b class="mp-num">{{ m.n }}</b>
      </div>
      <v-btn block variant="outlined" size="large" class="mt-5" :to="{ name: 'shopping' }">{{ t.openShopping }}</v-btn>
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
.hero-title { overflow-wrap: anywhere; }
.hero-facts { display: flex; flex-wrap: wrap; border-bottom: 1px solid var(--mp-rule); margin: -16px -16px 0; background: rgb(var(--v-theme-surface)); }
.hero-fact { flex: 1 1 50%; display: flex; align-items: center; gap: 8px; padding: 12px 16px; min-height: 56px; border-right: 1px solid var(--mp-line); }
.hero-fact:last-child { border-right: 0; }
.hero-actions { display: flex; flex-wrap: wrap; gap: 12px; padding-top: 16px; }
.hero-done { flex: 1 1 0; }
.strip { display: flex; gap: 6px; overflow-x: auto; scrollbar-width: none; padding: 2px; margin: 0 -2px; }
.day {
  flex: 1 0 44px; display: flex; flex-direction: column; align-items: center; gap: 2px; padding: 8px 0;
  border-radius: var(--mp-radius-control); background: transparent; border: 1px solid transparent;
  font: inherit; color: inherit; cursor: pointer;
}
.day span { font-size: 12px; color: rgba(var(--v-theme-on-surface), var(--v-medium-emphasis-opacity)); }
.day b { font-size: 18px; }
.day i { width: 18px; height: 2px; background: rgb(var(--v-theme-on-surface)); }
.day i.off { background: transparent; }
.day.today:not(.on) { border-color: rgb(var(--v-theme-primary)); }
.day.on { background: rgb(var(--v-theme-primary)); color: rgb(var(--v-theme-on-primary)); border-color: rgb(var(--v-theme-primary)); }
.day.on i { background: rgb(var(--v-theme-on-primary)); }
.day.on span { color: rgba(var(--v-theme-on-primary), 0.85); }
.section-label { font-size: 13px; font-weight: 700; letter-spacing: 0.8px; text-transform: uppercase; color: rgba(var(--v-theme-on-surface), var(--v-medium-emphasis-opacity)); margin: 8px 0; }
.day-rows { border-top: 1px solid var(--mp-rule); }
.meal { cursor: pointer; }
.meal.done .meal-title { text-decoration: line-through; opacity: 0.6; }
.meal-title {
  display: block; padding: 0; border: 0; background: none; color: inherit; cursor: pointer; text-align: start;
  font-family: var(--v-font-display); font-size: 1.125rem; font-weight: 600; line-height: 1.3;
}
.meal-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 12px; margin-top: 4px; font-size: 0.8125rem; opacity: 0.85; }
.done-btn { width: 44px; height: 44px; min-width: 44px; padding: 0; border-radius: var(--mp-radius-control); }
.free-btn {
  display: inline-flex; align-items: center; gap: 6px; padding: 8px 12px; min-height: 44px; background: transparent; color: inherit; font: inherit;
  border: 1px dashed rgba(var(--v-theme-on-surface), 0.55); border-radius: var(--mp-radius-control); cursor: pointer;
}
.free-hint { margin-top: 4px; font-size: 0.8125rem; opacity: 0.7; }
.quick-dock { position: sticky; bottom: calc(72px + 4px + env(safe-area-inset-bottom)); z-index: 3; }
@media (min-width: 960px) { .quick-dock { bottom: 24px; } }
.side-title { font-size: 1.125rem; padding-bottom: 6px; border-bottom: 1px solid var(--mp-rule); }
.week-row :deep(.note-row__label) { flex-basis: 72px; gap: 10px; }
.week-num { font-size: 1rem; }
.week-meals { font-size: 0.8125rem; opacity: 0.85; }
.cook-row { display: flex; align-items: center; justify-content: space-between; padding: 8px 0; }
</style>
