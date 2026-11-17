<script setup lang="ts">
import { portions } from '../polish'
import { computed, onMounted, ref } from 'vue'
import { useDisplay } from 'vuetify'
import { api } from '../api/client'
import type { PlanEntry, Slot } from '../api/types'
import EntryDialog from '../components/EntryDialog.vue'
import CookBadge from '../components/notebook/CookBadge.vue'
import NoteRow from '../components/notebook/NoteRow.vue'
import { errorText, t } from '../messages'
import { entryChip, entryTitle } from '../mealprep'
import { dayLine, dayWarnings } from '../nutrition'
import { useNutrition } from '../stores/nutrition'
import { usePlan, type EntryInput } from '../stores/plan'
import { useRecipes } from '../stores/recipes'
import { avatarColor } from '../today'
import { formatDay, isoDate, mondayOf, shiftWeek, SLOTS, weekDates } from '../weeks'

interface Member { id: string; displayName: string }

const { mdAndUp } = useDisplay()
const plan = usePlan()
const recipes = useRecipes()
const nutrition = useNutrition()
const members = ref<Member[]>([])
const week = ref(mondayOf(new Date()))
const error = ref('')
const ready = ref(false)
const notice = ref('')
const dialog = ref(false)
const editing = ref<PlanEntry | null>(null)
const initial = ref<{ date: string; slot: Slot }>({ date: week.value, slot: 'dinner' })

const dates = computed(() => weekDates(week.value))
const recipeTitle = computed(() => Object.fromEntries(recipes.items.map((r) => [r.id, r.title])))
const tracking = computed(() => nutrition.settings?.enabled === true)
const warnOf = (d: string) => (tracking.value && nutrition.days[d] ? dayWarnings(nutrition.days[d]).join(' · ') : '')
const todayIso = isoDate(new Date())
const isCurrentWeek = computed(() => week.value === mondayOf(new Date()))
const hasEntries = (d: string) => SLOTS.some((s) => plan.entriesFor(d, s.value).length > 0)
// An empty day has nothing to sum up, so it shows no "0 / 2200 kcal" line.
const lineOf = (d: string) => (tracking.value && nutrition.days[d] && hasEntries(d) ? dayLine(nutrition.days[d]) : '')
// A failed summary must not break the plan, so its errors are swallowed here.
async function refreshDays() {
  if (!tracking.value) return
  try {
    await nutrition.loadDays(dates.value[0], dates.value[6])
  } catch {
    /* the plan itself still works */
  }
}
const memberName = computed(() => Object.fromEntries(members.value.map((m) => [m.id, m.displayName])))
// An unknown cook (left the household) shows no badge rather than "undefined".
const cookName = (e: PlanEntry) => (e.cookUserId ? (memberName.value[e.cookUserId] ?? '') : '')
const color = (id: string) => avatarColor(id, members.value.map((m) => m.id))

async function load() {
  error.value = ''
  const target = week.value
  try {
    await plan.load(target)
    if (target === week.value) await refreshDays()
  } catch (e) {
    // a failure of a week the user already navigated away from is irrelevant
    if (target === week.value) error.value = errorText(e)
  }
}

async function init() {
  error.value = ''
  try {
    await Promise.all([recipes.load(), api<Member[]>('GET', '/household/members').then((m) => (members.value = m))])
    ready.value = true
  } catch (e) {
    error.value = errorText(e)
    return
  }
  try {
    await nutrition.loadSettings()
  } catch {
    /* nutrition stays hidden; the plan loads regardless */
  }
  await load()
}

onMounted(init)

async function go(n: number) {
  notice.value = ''
  week.value = shiftWeek(week.value, n)
  await load()
}

async function today() {
  notice.value = ''
  week.value = mondayOf(new Date())
  await load()
}

function retry() {
  return ready.value ? load() : init()
}

const editable = computed(() => ready.value && plan.loaded && plan.week === week.value)

function label(e: PlanEntry): string {
  return entryTitle(e, e.recipeId ? (recipeTitle.value[e.recipeId] ?? '…') : null)
}

function openNew(date: string, slot: Slot) {
  editing.value = null
  initial.value = { date, slot }
  dialog.value = true
}

function openEdit(e: PlanEntry) {
  editing.value = e
  dialog.value = true
}

async function save(input: EntryInput) {
  error.value = ''
  try {
    if (editing.value) await plan.edit(editing.value.id, input)
    else await plan.add(input)
    dialog.value = false
    await refreshDays()
  } catch (e) {
    error.value = errorText(e)
  }
}

async function removeEntry() {
  if (!editing.value) return
  error.value = ''
  try {
    await plan.remove(editing.value.id)
    dialog.value = false
    await refreshDays()
  } catch (e) {
    error.value = errorText(e)
  }
}

async function copyPrev() {
  error.value = ''
  notice.value = ''
  try {
    await plan.copyFrom(shiftWeek(week.value, -1))
    notice.value = t.weekCopied
    await refreshDays()
  } catch (e) {
    error.value = errorText(e)
  }
}
</script>

<template>
  <div class="d-flex align-center flex-wrap ga-2 mb-4">
    <v-btn icon="mdi-chevron-left" variant="text" :title="t.prevWeek" :aria-label="t.prevWeek" @click="go(-1)" />
    <h1 class="mp-display text-h6">{{ formatDay(dates[0]) }} – {{ formatDay(dates[6]) }}</h1>
    <v-btn icon="mdi-chevron-right" variant="text" :title="t.nextWeek" :aria-label="t.nextWeek" @click="go(1)" />
    <v-btn v-if="!isCurrentWeek" variant="tonal" @click="today">{{ t.thisWeek }}</v-btn>
    <v-spacer />
    <v-btn variant="outlined" prepend-icon="mdi-content-copy" :disabled="!editable" @click="copyPrev">{{ t.copyPrevWeek }}</v-btn>
  </div>
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">
    {{ error }}
    <template v-if="!editable" #append><v-btn variant="text" @click="retry">{{ t.retry }}</v-btn></template>
  </v-alert>
  <v-alert v-if="notice" type="success" variant="tonal" class="mb-4">{{ notice }}</v-alert>

  <v-progress-linear v-if="!editable && !error" indeterminate class="mb-4" />

  <template v-if="editable">
  <div v-if="mdAndUp" class="plan-scroll">
    <table class="plan-table">
      <thead>
        <tr>
          <th scope="col">{{ t.planDayColumn }}</th>
          <th v-for="s in SLOTS" :key="s.value" scope="col">{{ s.title }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="d in dates" :key="d" :class="{ 'plan-row--today': d === todayIso }" :aria-current="d === todayIso ? 'date' : undefined">
          <th scope="row" class="plan-day-head">
            <div class="plan-day-name">{{ formatDay(d) }}<span v-if="d === todayIso" class="today-tag">{{ t.today }}</span></div>
            <div v-if="lineOf(d)" class="day-line text-medium-emphasis">
              <span class="mp-num">{{ lineOf(d) }}</span>
              <v-icon v-if="warnOf(d)" icon="mdi-alert-outline" size="x-small" color="warning" :title="warnOf(d)" :aria-label="warnOf(d)" />
            </div>
          </th>
          <td v-for="s in SLOTS" :key="s.value">
            <button v-for="e in plan.entriesFor(d, s.value)" :key="e.id" type="button" class="plan-entry" @click="openEdit(e)">
              <span class="plan-entry__title">{{ label(e) }}</span>
              <span class="plan-entry__meta text-medium-emphasis">
                <CookBadge v-if="cookName(e)" :name="cookName(e)" :color="color(e.cookUserId!)" />
                {{ portions(e.servings) }}
              </span>
              <span v-if="entryChip(e)" class="plan-entry__note text-medium-emphasis">{{ entryChip(e) }}</span>
            </button>
            <v-btn
              v-if="plan.entriesFor(d, s.value).length === 0"
              variant="text"
              size="small"
              color="primary"
              prepend-icon="mdi-plus"
              :title="`${t.addEntryTitle}: ${formatDay(d)}, ${s.title}`"
              :aria-label="`${t.addEntryTitle}: ${formatDay(d)}, ${s.title}`"
              @click="openNew(d, s.value)"
            >{{ t.addEntryTitle }}</v-btn>
            <v-btn
              v-else
              icon="mdi-plus"
              size="x-small"
              variant="text"
              color="primary"
              :title="`${t.addEntryTitle}: ${formatDay(d)}, ${s.title}`"
              :aria-label="`${t.addEntryTitle}: ${formatDay(d)}, ${s.title}`"
              @click="openNew(d, s.value)"
            />
          </td>
        </tr>
      </tbody>
    </table>
  </div>

  <div v-else>
    <section v-for="d in dates" :key="d" class="plan-day" :class="{ 'plan-day--today': d === todayIso }" :aria-current="d === todayIso ? 'date' : undefined">
      <header class="plan-day__head">
        <div class="plan-day-name">{{ formatDay(d) }}<span v-if="d === todayIso" class="today-tag">{{ t.today }}</span></div>
        <div v-if="lineOf(d)" class="day-line text-medium-emphasis">
          <span class="mp-num">{{ lineOf(d) }}</span>
          <v-icon v-if="warnOf(d)" icon="mdi-alert-outline" size="small" color="warning" :title="warnOf(d)" :aria-label="warnOf(d)" />
          <span v-if="warnOf(d)" class="d-block">{{ warnOf(d) }}</span>
        </div>
      </header>
      <template v-for="s in SLOTS" :key="s.value">
        <NoteRow v-for="e in plan.entriesFor(d, s.value)" :key="e.id" :label="s.title" :highlighted="d === todayIso">
          <button type="button" class="plan-entry" @click="openEdit(e)">
            <span class="plan-entry__title">{{ label(e) }}</span>
            <span class="plan-entry__meta text-medium-emphasis">
              <CookBadge v-if="cookName(e)" :name="cookName(e)" :color="color(e.cookUserId!)" />
              {{ portions(e.servings) }}
            </span>
            <span v-if="entryChip(e)" class="plan-entry__note text-medium-emphasis">{{ entryChip(e) }}</span>
          </button>
        </NoteRow>
      </template>
      <v-btn variant="text" color="primary" prepend-icon="mdi-plus" :aria-label="`${t.addEntryTitle}: ${formatDay(d)}`" @click="openNew(d, 'dinner')">{{ t.addEntryTitle }}</v-btn>
    </section>
  </div>
  </template>

  <EntryDialog
    v-model="dialog"
    :dates="dates"
    :recipes="recipes.items"
    :members="members"
    :entry="editing"
    :initial="initial"
    @save="save"
    @remove="removeEntry"
  />
</template>

<style scoped>
.plan-scroll { overflow-x: auto; }
.plan-table { width: 100%; border-collapse: collapse; table-layout: fixed; }
.plan-table th, .plan-table td { border-bottom: 1px solid var(--mp-line); padding: 8px 12px; text-align: left; vertical-align: top; }
.plan-table thead th { border-bottom: 1px solid var(--mp-rule); font-size: 0.8125rem; font-weight: 600; }
.plan-table th:first-child { width: 140px; }
.plan-day-head { font-weight: 600; }
.plan-row--today { background: rgb(var(--v-theme-surface-variant)); }
.day-line { font-size: 12px; font-weight: 400; white-space: normal; }
.plan-day { margin-bottom: 16px; border-top: 1px solid var(--mp-rule); }
.plan-day--today { background: rgb(var(--v-theme-surface-variant)); }
.plan-day__head { padding: 8px 12px 4px; font-weight: 600; }
.plan-entry { display: block; width: 100%; padding: 0; margin-bottom: 4px; border: 0; background: none; color: inherit; font: inherit; text-align: left; cursor: pointer; }
.plan-entry__title { display: block; text-decoration: underline; text-underline-offset: 2px; }
.plan-entry__meta { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; font-size: 0.8125rem; }
.plan-entry__note { display: block; font-size: 0.75rem; }
.today-tag {
  display: inline-block; margin-left: 6px; padding: 1px 8px; border-radius: 999px; font-size: 11px; font-weight: 700;
  background: rgb(var(--v-theme-primary)); color: rgb(var(--v-theme-on-primary)); vertical-align: middle;
}
</style>
