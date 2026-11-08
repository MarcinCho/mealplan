<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useDisplay } from 'vuetify'
import { api } from '../api/client'
import type { PlanEntry, Slot } from '../api/types'
import EntryDialog from '../components/EntryDialog.vue'
import { errorText, t } from '../messages'
import { dayLine, dayWarnings } from '../nutrition'
import { useNutrition } from '../stores/nutrition'
import { usePlan, type EntryInput } from '../stores/plan'
import { useRecipes } from '../stores/recipes'
import { formatDay, mondayOf, shiftWeek, SLOTS, weekDates } from '../weeks'

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
const lineOf = (d: string) => (tracking.value && nutrition.days[d] ? dayLine(nutrition.days[d]) : '')
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
  return e.recipeId ? (recipeTitle.value[e.recipeId] ?? '…') : e.freeText
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
    <v-btn icon="mdi-chevron-left" variant="text" :title="t.prevWeek" @click="go(-1)" />
    <h1 class="text-h6">{{ formatDay(dates[0]) }} – {{ formatDay(dates[6]) }}</h1>
    <v-btn icon="mdi-chevron-right" variant="text" :title="t.nextWeek" @click="go(1)" />
    <v-btn variant="text" @click="today">{{ t.thisWeek }}</v-btn>
    <v-spacer />
    <v-btn variant="tonal" prepend-icon="mdi-content-copy" :disabled="!editable" @click="copyPrev">{{ t.copyPrevWeek }}</v-btn>
  </div>
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">
    {{ error }}
    <template v-if="!editable" #append><v-btn variant="text" @click="retry">{{ t.retry }}</v-btn></template>
  </v-alert>
  <v-alert v-if="notice" type="success" variant="tonal" class="mb-4">{{ notice }}</v-alert>

  <v-progress-linear v-if="!editable && !error" indeterminate class="mb-4" />

  <template v-if="editable">
  <v-table v-if="mdAndUp" class="plan-grid">
    <thead>
      <tr>
        <th />
        <th v-for="d in dates" :key="d">
          {{ formatDay(d) }}
          <div v-if="lineOf(d)" class="day-line">
            {{ lineOf(d) }}
            <v-icon v-if="warnOf(d)" icon="mdi-alert-outline" size="x-small" color="warning" :title="warnOf(d)" :aria-label="warnOf(d)" />
          </div>
        </th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="s in SLOTS" :key="s.value">
        <th class="text-left">{{ s.title }}</th>
        <td v-for="d in dates" :key="d" class="align-top pa-1">
          <v-chip
            v-for="e in plan.entriesFor(d, s.value)"
            color="primary"
            variant="tonal"
            :key="e.id"
            class="mb-1 d-flex"
            style="height: auto; white-space: normal"
            @click="openEdit(e)"
          >
            <span>{{ label(e) }}<template v-if="e.cookUserId"> · {{ memberName[e.cookUserId] }}</template></span>
          </v-chip>
          <v-btn icon="mdi-plus" size="x-small" variant="text" color="primary" :title="t.addEntry" @click="openNew(d, s.value)" />
        </td>
      </tr>
    </tbody>
  </v-table>

  <div v-else>
    <v-card v-for="d in dates" :key="d" class="mb-3">
      <v-card-title class="text-subtitle-1">{{ formatDay(d) }}</v-card-title>
      <v-card-subtitle v-if="lineOf(d)" class="text-wrap">
        {{ lineOf(d) }}
        <v-icon v-if="warnOf(d)" icon="mdi-alert-outline" size="small" color="warning" :title="warnOf(d)" :aria-label="warnOf(d)" />
        <span v-if="warnOf(d)" class="d-block">{{ warnOf(d) }}</span>
      </v-card-subtitle>
      <v-list density="compact" bg-color="transparent">
        <template v-for="s in SLOTS" :key="s.value">
          <v-list-item
            v-for="e in plan.entriesFor(d, s.value)"
            :key="e.id"
            :title="label(e)"
            :subtitle="`${s.title} · ${e.servings} ${t.servingsShort}${e.cookUserId ? ' · ' + memberName[e.cookUserId] : ''}`"
            @click="openEdit(e)"
          />
        </template>
      </v-list>
      <v-card-actions>
        <v-btn size="small" variant="text" prepend-icon="mdi-plus" @click="openNew(d, 'dinner')">{{ t.addEntry }}</v-btn>
      </v-card-actions>
    </v-card>
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
.plan-grid td { min-width: 120px; }
.day-line { font-size: 11px; font-weight: 400; opacity: 0.7; white-space: normal; }
</style>
