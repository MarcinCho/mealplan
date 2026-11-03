import { ApiError } from './api/client'
import type { Recipe, RecipeDraft } from './api/types'
import { errorText, t } from './messages'

export function draftToRecipe(d: RecipeDraft, id = ''): Recipe {
  return {
    id,
    title: d.title,
    baseServings: d.servings > 0 ? d.servings : 4,
    sourceUrl: d.sourceUrl,
    tags: [...d.tags],
    // An unparsed line keeps its text as the name so nothing is lost; the highlighted row asks the user to fix it.
    ingredients: d.ingredients.map((i) => ({
      name: i.confidence === 'unparsed' && !i.name ? Array.from(i.raw).slice(0, 200).join('') : i.name,
      quantity: i.quantity,
      unit: i.unit,
      raw: i.raw,
      note: i.note,
      confidence: i.confidence,
    })),
    steps: [...d.steps],
  }
}

export function normalizeUrl(u: string): string {
  return u
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, '')
    .replace(/^www\./, '')
    .replace(/#.*$/, '')
    .replace(/\/+$/, '')
}

export function findDuplicate(d: RecipeDraft, existing: Recipe[]): Recipe | undefined {
  const url = normalizeUrl(d.sourceUrl)
  const title = d.title.trim().toLowerCase()
  return existing.find(
    (r) => (url !== '' && normalizeUrl(r.sourceUrl) === url) || (title !== '' && r.title.trim().toLowerCase() === title),
  )
}

// Routes that consume the import draft; leaving the import screen anywhere else must drop it.
export function keepsDraft(name: unknown): boolean {
  return name === 'recipe-new' || name === 'recipe-edit'
}

// "Update existing": the draft replaces title, ingredients and steps but must not erase what it did not carry.
export function mergeIntoExisting(d: RecipeDraft, existing: Recipe): Recipe {
  const merged = draftToRecipe(d, existing.id)
  if (!d.title.trim()) merged.title = existing.title
  if (!(d.servings > 0)) merged.baseServings = existing.baseServings
  if (!merged.sourceUrl) merged.sourceUrl = existing.sourceUrl
  if (!merged.tags.length) merged.tags = [...existing.tags]
  if (!merged.ingredients.length) merged.ingredients = existing.ingredients.map((i) => ({ ...i }))
  if (!merged.steps.length) merged.steps = [...existing.steps]
  return merged
}

// Links typed without a scheme ("example.com/zupa") are sent as https.
export function normalizeImportUrl(u: string): string {
  const s = u.trim()
  return s !== '' && !/^[a-z][a-z0-9+.-]*:/i.test(s) ? `https://${s}` : s
}

export function importErrorText(e: unknown, tab: 'url' | 'text'): string {
  if (e instanceof ApiError) {
    if (e.code === 'invalid_input') {
      const m = e.message.replace(/^invalid input:\s*/, '')
      const sentence = m.charAt(0).toUpperCase() + m.slice(1)
      return /[.!?]$/.test(sentence) ? sentence : `${sentence}.`
    }
    if (e.code === 'import_unparsable' && tab === 'text') return t.importTextUnparsable
  }
  return errorText(e)
}
