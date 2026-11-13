// Interface language. Polish is the default; English is chosen per browser on the account page.
// Switching reloads the page, so every module reads one fixed language for its lifetime.
export type Locale = 'pl' | 'en'

export const LOCALES: { value: Locale; title: string }[] = [
  { value: 'pl', title: 'Polski' },
  { value: 'en', title: 'English' },
]

const KEY = 'mealplan.locale'

function stored(): Locale {
  try {
    return localStorage.getItem(KEY) === 'en' ? 'en' : 'pl'
  } catch {
    return 'pl'
  }
}

export const locale: Locale = stored()

// BCP 47 tag for Intl formatting of dates and numbers.
export const intl = locale === 'en' ? 'en-GB' : 'pl-PL'

export function setLocale(l: Locale, reload: () => void = () => location.reload()): void {
  if (l === locale) return
  try {
    localStorage.setItem(KEY, l)
  } catch {
    return
  }
  reload()
}
