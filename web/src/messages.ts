import { ApiError } from './api/client'
import { locale } from './i18n'
import { en } from './locales/en'
import { pl, type Messages } from './locales/pl'

// Every text the interface shows, in the language chosen for this browser (see i18n.ts).
export const t: Messages = locale === 'en' ? en : pl

export function movedText(n: number): string {
  return t.moved(n)
}

export function recipeSavedText(title: string): string {
  return t.recipeSaved(title.trim())
}

export function recipeDeletedText(title: string): string {
  return t.recipeDeleted(title.trim())
}

export function undonePreviousText(n: number): string {
  return t.undonePrevious(n)
}

export function pantryErrorText(e: unknown): string {
  if (e instanceof ApiError && t.pantryErrors[e.code]) return t.pantryErrors[e.code]
  return errorText(e)
}

// Like errorText, but with wording that fits the product database.
export function productErrorText(e: unknown): string {
  if (e instanceof ApiError && t.productErrors[e.code]) return t.productErrors[e.code]
  return errorText(e)
}

export function errorText(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.code === 'rate_limited') return t.rateLimited(e.retryAfter ?? null)
    return t.errors[e.code] ?? e.message
  }
  return t.genericError
}

// Like errorText, but with wording that fits meal prep deletes.
export function prepErrorText(e: unknown): string {
  if (e instanceof ApiError && e.code === 'in_use') return t.prepInUse
  return errorText(e)
}

// "3 przepisy · 1 w spiżarni · 2 w dzienniku": only the places where the product is used.
export function usageLine(u: { recipes: number; pantry: number; foodLog: number; shopping: number }): string {
  const parts = [
    u.recipes ? t.usageRecipes(u.recipes) : '',
    u.pantry ? t.usagePantry(u.pantry) : '',
    u.foodLog ? t.usageFoodLog(u.foodLog) : '',
    u.shopping ? t.usageShopping(u.shopping) : '',
  ].filter(Boolean)
  return parts.length ? parts.join(' · ') : t.usageNone
}

export function mergedNotice(name: string, skippedAliases: string[]): string {
  return t.merged(name, skippedAliases)
}
