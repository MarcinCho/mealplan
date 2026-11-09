<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { FreezerItem, PrepDish, PrepSession } from '../api/types'
import AllocateDialog, { type AllocateSource } from '../components/AllocateDialog.vue'
import EmptyState from '../components/EmptyState.vue'
import PrepDishCard from '../components/PrepDishCard.vue'
import { defaultWeek, sessionTitle, staleSessions } from '../mealprep'
import { errorText, t } from '../messages'
import { useMealPrepStore } from '../stores/mealprep'
import { useRecipes } from '../stores/recipes'
import { isoDate, mondayOf, shiftWeek } from '../weeks'

const prep = useMealPrepStore()
const recipes = useRecipes()
const tab = ref<'sessions' | 'freezer'>('sessions')
const error = ref('')
const today = isoDate(new Date())

async function run(fn: () => Promise<unknown>): Promise<boolean> {
  error.value = ''
  try {
    await fn()
    return true
  } catch (e) {
    error.value = errorText(e)
    return false
  }
}
const load = () => run(() => Promise.all([prep.load(), recipes.load()]))
onMounted(load)

const stale = computed(() => staleSessions(prep.sessions, today))
const sorted = computed(() => [...prep.sessions].sort((a, b) => b.date.localeCompare(a.date)))
const nl = (d: string) => new Date(d + 'T00:00:00').toLocaleDateString('pl-PL', { day: 'numeric', month: 'long' })
const weekLabel = (w: string) => `${t.prepWeekFrom} ${nl(w)}`

// Which dialog is open; one at a time.
type Dlg =
  | { k: 'session'; date: string; name: string; week: string; touchedName: boolean; touchedWeek: boolean }
  | { k: 'dish'; sessionId: string; recipeId: string | null; portions: number }
  | { k: 'editDish'; dish: PrepDish; portions: number; writtenOff: number }
  | { k: 'freeze'; dish: PrepDish; portions: number }
  | { k: 'freezer'; id: string | null; recipeId: string | null; label: string; portions: number; frozenOn: string }
  | { k: 'confirm'; text: string; fn: () => Promise<unknown> }
const dlg = ref<Dlg | null>(null)
const open = computed({ get: () => dlg.value !== null, set: (v) => { if (!v) dlg.value = null } })
const alloc = ref<AllocateSource | null>(null)
const allocOpen = computed({ get: () => alloc.value !== null, set: (v) => { if (!v) alloc.value = null } })

function newSession() {
  dlg.value = { k: 'session', date: today, name: sessionTitle(today), week: defaultWeek(today), touchedName: false, touchedWeek: false }
}
function onSessionDate(d: string) {
  const x = dlg.value
  if (x?.k !== 'session') return
  x.date = d
  if (!/^\d{4}-\d{2}-\d{2}$/.test(d)) return
  if (!x.touchedName) x.name = sessionTitle(d)
  if (!x.touchedWeek) x.week = defaultWeek(d)
}
const weekItems = computed(() => {
  const x = dlg.value
  if (x?.k !== 'session' || !/^\d{4}-\d{2}-\d{2}$/.test(x.date)) return []
  const base = mondayOf(new Date(x.date + 'T00:00:00'))
  const set = new Set([shiftWeek(base, -1), base, shiftWeek(base, 1), shiftWeek(base, 2), x.week])
  return [...set].sort().map((w) => ({ title: weekLabel(w), value: w }))
})
const recipeItems = computed(() => [...recipes.items].sort((a, b) => a.title.localeCompare(b.title, 'pl')))

const posInt = (n: unknown) => typeof n === 'number' && Number.isInteger(n) && n >= 1
const valid = computed(() => {
  const x = dlg.value
  if (!x) return false
  switch (x.k) {
    case 'session': return /^\d{4}-\d{2}-\d{2}$/.test(x.date) && x.name.trim() !== '' && !!x.week
    case 'dish': return !!x.recipeId && posInt(x.portions)
    case 'editDish': return posInt(x.portions) && Number.isInteger(x.writtenOff) && x.writtenOff >= 0
    case 'freeze': return posInt(x.portions) && x.portions <= x.dish.remaining
    case 'freezer': return (!!x.recipeId || x.label.trim() !== '') && posInt(x.portions)
    default: return true
  }
})

async function submit() {
  const x = dlg.value
  if (!x || !valid.value) return
  const ok = await run(() => {
    switch (x.k) {
      case 'session': return prep.createSession({ date: x.date, name: x.name.trim(), week: x.week })
      case 'dish': return prep.addDish(x.sessionId, x.recipeId!, x.portions)
      case 'editDish': return prep.updateDish(x.dish.id, x.portions, x.writtenOff)
      case 'freeze': return prep.freeze(x.dish.id, x.portions)
      case 'freezer': {
        const input = { recipeId: x.recipeId, label: x.label.trim(), portions: x.portions, frozenOn: x.frozenOn }
        return x.id ? prep.updateFreezer(x.id, input) : prep.addFreezer(input)
      }
      case 'confirm': return x.fn()
    }
  })
  if (ok) dlg.value = null
}

const setCooked = (s: PrepSession, cooked: boolean | null) =>
  run(() => prep.updateSession(s.id, { date: s.date, name: s.name, week: s.week, cooked: Boolean(cooked) }))
const confirm = (text: string, fn: () => Promise<unknown>) => (dlg.value = { k: 'confirm', text, fn })
const allocDish = (d: PrepDish) => (alloc.value = { dishId: d.id, title: d.recipeTitle, remaining: d.remaining })
const allocFreezer = (f: FreezerItem) => (alloc.value = { freezerItemId: f.id, title: f.title, remaining: f.remaining })
const addFreezer = () => (dlg.value = { k: 'freezer', id: null, recipeId: null, label: '', portions: 1, frozenOn: today })
const editFreezer = (f: FreezerItem) =>
  (dlg.value = { k: 'freezer', id: f.id, recipeId: f.recipeId, label: f.title, portions: f.portions, frozenOn: f.frozenOn })
</script>

<template>
  <div class="d-flex align-center flex-wrap ga-2 mb-2">
    <h1 class="text-h5">{{ t.prepTitle }}</h1>
    <v-spacer />
    <v-btn v-if="tab === 'sessions'" color="primary" prepend-icon="mdi-plus" @click="newSession">{{ t.prepNewSession }}</v-btn>
    <v-btn v-else color="primary" prepend-icon="mdi-plus" @click="addFreezer">{{ t.freezerAdd }}</v-btn>
  </div>
  <v-tabs v-model="tab" color="primary" class="mb-4">
    <v-tab value="sessions">{{ t.prepTabSessions }}</v-tab>
    <v-tab value="freezer">{{ t.prepTabFreezer }}</v-tab>
  </v-tabs>

  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">
    {{ error }}
    <template v-if="!prep.loaded" #append><v-btn variant="text" @click="load">{{ t.retry }}</v-btn></template>
  </v-alert>
  <v-progress-linear v-if="!prep.loaded && !error" indeterminate class="mb-4" />

  <template v-if="tab === 'sessions'">
    <v-alert v-if="stale.length" type="warning" variant="tonal" class="mb-4">{{ t.prepStale }}</v-alert>
    <EmptyState v-if="prep.loaded && !prep.sessions.length" :title="t.prepEmpty" :hint="t.prepEmptyHint">
      <v-btn color="primary" @click="newSession">{{ t.prepNewSession }}</v-btn>
    </EmptyState>
    <v-card v-for="s in sorted" :key="s.id" class="pa-3 mb-4">
      <div class="d-flex align-center flex-wrap ga-2">
        <div class="flex-grow-1">
          <div class="text-subtitle-1 font-weight-bold">{{ s.name }}</div>
          <div class="text-body-2 text-medium-emphasis">{{ nl(s.date) }} · {{ weekLabel(s.week) }}</div>
        </div>
        <v-switch :model-value="s.cooked" :label="t.prepCooked" density="compact" color="primary" hide-details @update:model-value="setCooked(s, $event)" />
        <v-btn icon="mdi-delete-outline" size="small" variant="text" :title="t.prepSessionDelete" @click="confirm(`${t.prepSessionDelete}: ${s.name}?`, () => prep.removeSession(s.id))" />
      </div>
      <div class="mt-3">
        <PrepDishCard
          v-for="d in s.dishes"
          :key="d.id"
          :dish="d"
          @allocate="allocDish(d)"
          @freeze="dlg = { k: 'freeze', dish: d, portions: Math.min(d.remaining, 1) }"
          @writeoff="dlg = { k: 'editDish', dish: d, portions: d.portions, writtenOff: d.writtenOff }"
          @edit="dlg = { k: 'editDish', dish: d, portions: d.portions, writtenOff: d.writtenOff }"
          @remove="confirm(`${t.prepDishDelete}: ${d.recipeTitle}?`, () => prep.removeDish(d.id))"
        />
        <v-btn variant="text" prepend-icon="mdi-plus" @click="dlg = { k: 'dish', sessionId: s.id, recipeId: null, portions: 4 }">{{ t.prepAddDish }}</v-btn>
      </div>
    </v-card>
  </template>

  <template v-else>
    <EmptyState v-if="prep.loaded && !prep.freezer.length" :title="t.freezerEmpty" :hint="t.freezerEmptyHint">
      <v-btn color="primary" @click="addFreezer">{{ t.freezerAdd }}</v-btn>
    </EmptyState>
    <v-card v-for="f in prep.freezer" :key="f.id" class="pa-3 mb-2">
      <div class="d-flex align-center flex-wrap ga-2">
        <div class="flex-grow-1">
          <div class="font-weight-bold">{{ f.title }}</div>
          <div class="text-body-2 text-medium-emphasis">{{ f.remaining }} {{ t.freezerLeft }} · {{ t.freezerFrozenOn }} {{ nl(f.frozenOn) }}</div>
        </div>
        <v-btn size="small" color="primary" variant="tonal" :disabled="f.remaining < 1" @click="allocFreezer(f)">{{ t.prepAllocate }}</v-btn>
        <v-btn icon="mdi-pencil-outline" size="small" variant="text" :title="t.prepEditDish" @click="editFreezer(f)" />
        <v-btn icon="mdi-delete-outline" size="small" variant="text" :title="t.delete" @click="confirm(`${t.delete}: ${f.title}?`, () => prep.removeFreezer(f.id))" />
      </div>
    </v-card>
  </template>

  <AllocateDialog v-if="alloc" v-model="allocOpen" :source="alloc" />

  <v-dialog v-model="open" max-width="480">
    <v-card v-if="dlg">
      <v-card-title>
        <template v-if="dlg.k === 'session'">{{ t.prepNewSession }}</template>
        <template v-else-if="dlg.k === 'dish'">{{ t.prepAddDish }}</template>
        <template v-else-if="dlg.k === 'editDish'">{{ dlg.dish.recipeTitle }}</template>
        <template v-else-if="dlg.k === 'freeze'">{{ t.prepFreeze }}</template>
        <template v-else-if="dlg.k === 'freezer'">{{ dlg.id ? t.prepEditDish : t.freezerAdd }}</template>
        <template v-else>{{ t.delete }}</template>
      </v-card-title>
      <v-card-text>
        <template v-if="dlg.k === 'session'">
          <v-text-field :model-value="dlg.date" type="date" :label="t.prepSessionDate" @update:model-value="onSessionDate($event)" />
          <v-text-field v-model="dlg.name" :label="t.prepSessionName" maxlength="100" @update:model-value="dlg.touchedName = true" />
          <v-select v-model="dlg.week" :items="weekItems" :label="t.prepSessionWeek" @update:model-value="dlg.touchedWeek = true" />
        </template>
        <template v-else-if="dlg.k === 'dish'">
          <v-select v-model="dlg.recipeId" :items="recipeItems" item-title="title" item-value="id" :label="t.prepRecipe" />
          <v-text-field v-model.number="dlg.portions" type="number" min="1" max="200" :label="t.prepPortions" />
        </template>
        <template v-else-if="dlg.k === 'editDish'">
          <v-text-field v-model.number="dlg.portions" type="number" min="1" max="200" :label="t.prepPortions" />
          <v-text-field v-model.number="dlg.writtenOff" type="number" min="0" :label="t.prepWrittenOff" />
        </template>
        <template v-else-if="dlg.k === 'freeze'">
          <v-text-field v-model.number="dlg.portions" type="number" min="1" :max="dlg.dish.remaining" :label="t.prepFreezePortions" />
        </template>
        <template v-else-if="dlg.k === 'freezer'">
          <v-select v-model="dlg.recipeId" :items="recipeItems" item-title="title" item-value="id" :label="t.prepRecipe" clearable />
          <v-text-field v-model="dlg.label" :label="t.freezerLabel" maxlength="100" />
          <v-text-field v-model.number="dlg.portions" type="number" min="1" max="200" :label="t.prepPortions" />
          <v-text-field v-model="dlg.frozenOn" type="date" :label="t.freezerFrozenOn" />
        </template>
        <template v-else>{{ dlg.text }}</template>
        <v-alert v-if="error" type="error" variant="tonal" density="compact" class="mt-2">{{ error }}</v-alert>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="dlg = null">{{ t.cancel }}</v-btn>
        <v-btn :color="dlg.k === 'confirm' ? 'error' : 'primary'" :disabled="!valid" @click="submit">{{ dlg.k === 'confirm' ? t.delete : t.save }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
