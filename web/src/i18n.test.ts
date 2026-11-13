import { afterEach, describe, expect, it, vi } from 'vitest'
import { en } from './locales/en'
import { pl } from './locales/pl'

// Every string the English dictionary holds, functions called with sample arguments.
function strings(v: unknown): string[] {
  if (typeof v === 'string') return [v]
  if (typeof v === 'function') {
    // Counts first ((n, unit), (n)), then names ((name, other), (name, list)).
    const calls = [[2, 'x'], [1, 'x'], ['x', 'y'], ['x', ['y']]]
    return calls.flatMap((args) => {
      try {
        return [String(v(...args))]
      } catch {
        return []
      }
    })
  }
  if (Array.isArray(v)) return v.flatMap(strings)
  if (v && typeof v === 'object') return Object.values(v).flatMap(strings)
  return []
}

afterEach(() => {
  localStorage.clear()
  vi.resetModules()
})

describe('i18n', () => {
  it('has an English text for every Polish one', () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(pl).sort())
    expect(Object.keys(en.errors).sort()).toEqual(Object.keys(pl.errors).sort())
  })

  it('keeps Polish out of the English texts', () => {
    const polish = strings(en).filter((s) => /[ąćęłńóśźż]/i.test(s))
    expect(polish).toEqual([])
  })

  it('defaults to Polish and switches to English from storage', async () => {
    expect((await import('./messages')).t.navPlan).toBe('Plan')
    expect((await import('./messages')).t.navRecipes).toBe('Przepisy')
    vi.resetModules()
    localStorage.setItem('mealplan.locale', 'en')
    const m = await import('./messages')
    expect(m.t.navRecipes).toBe('Recipes')
    const p = await import('./polish')
    expect(p.portions(2)).toBe('2 portions')
    expect(p.portions(1.5)).toBe('1.5 portions')
    expect(`2 ${p.unitWord(2, 'łyżka')}`).toBe('2 tbsp')
    expect(p.shortDate('2026-11-12', new Date(2026, 0, 1))).toBe('12 Nov')
  })

  it('stores the choice and reloads only when the language changes', async () => {
    const { setLocale } = await import('./i18n')
    const reload = vi.fn()
    setLocale('pl', reload)
    expect(reload).not.toHaveBeenCalled()
    setLocale('en', reload)
    expect(localStorage.getItem('mealplan.locale')).toBe('en')
    expect(reload).toHaveBeenCalledOnce()
  })
})
