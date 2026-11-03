import type { Recipe, RecipeDraft } from './api/types'

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
