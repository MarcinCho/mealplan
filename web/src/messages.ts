import { ApiError } from './api/client'

export const t = {
  appName: 'Mealplan',
  login: 'Login',
  password: 'Hasło',
  loginHint: 'Małe litery a-z, cyfry oraz znaki . _ - (od 3 do 64 znaków)',
  passwordHint: 'Co najmniej 12 znaków',
  displayName: 'Imię',
  householdName: 'Nazwa gospodarstwa',
  signIn: 'Zaloguj',
  signOut: 'Wyloguj',
  setupTitle: 'Pierwsze uruchomienie',
  setupIntro: 'Utwórz gospodarstwo i konto administratora.',
  createAccount: 'Utwórz konto',
  inviteTitle: 'Dołącz do gospodarstwa',
  inviteInvalid: 'To zaproszenie jest nieprawidłowe, zostało użyte lub wygasło.',
  navPlan: 'Plan',
  navMembers: 'Domownicy',
  homeSoon: 'Plan tygodnia pojawi się w kolejnym etapie.',
  members: 'Domownicy',
  admin: 'Administrator',
  member: 'Domownik',
  invites: 'Zaproszenia',
  newInvite: 'Nowe zaproszenie',
  role: 'Rola',
  createInvite: 'Utwórz link',
  copy: 'Kopiuj',
  copied: 'Skopiowano',
  copyFailed: 'Nie udało się skopiować automatycznie. Zaznacz link i skopiuj go ręcznie.',
  inviteLinkOnce: 'Link jest pokazywany tylko raz. Skopiuj go teraz.',
  noInvites: 'Brak oczekujących zaproszeń.',
  expires: 'Ważne do',
  revoke: 'Unieważnij',
  theme: 'Zmień motyw',
  navRecipes: 'Przepisy',
  navShopping: 'Zakupy',
  recipes: 'Przepisy',
  newRecipe: 'Nowy przepis',
  editRecipe: 'Edytuj przepis',
  recipeTitle: 'Tytuł',
  baseServings: 'Porcje bazowe',
  sourceUrl: 'Adres źródła (opcjonalnie)',
  tags: 'Tagi',
  ingredients: 'Składniki',
  ingredientName: 'Nazwa',
  quantity: 'Ilość',
  unit: 'Jednostka',
  addIngredient: 'Dodaj składnik',
  steps: 'Kroki',
  addStep: 'Dodaj krok',
  save: 'Zapisz',
  cancel: 'Anuluj',
  delete: 'Usuń',
  confirmDelete: 'Na pewno usunąć?',
  noRecipes: 'Nie masz jeszcze przepisów.',
  servingsShort: 'porcji',
  remove: 'Usuń',
  loading: 'Ładowanie…',
}

const errors: Record<string, string> = {
  invalid_credentials: 'Nieprawidłowy login lub hasło.',
  setup_done: 'Konfiguracja została już wykonana.',
  invite_invalid: t.inviteInvalid,
  login_taken: 'Ten login jest już zajęty.',
  password_length: 'Hasło musi mieć od 12 do 128 znaków.',
  invalid_input: 'Sprawdź poprawność wpisanych danych.',
  forbidden: 'Brak uprawnień do tej operacji.',
  unauthenticated: 'Sesja wygasła. Zaloguj się ponownie.',
  csrf: 'Sesja jest nieaktualna. Odśwież stronę i spróbuj ponownie.',
  not_found: 'Nie znaleziono żądanego zasobu.',
  too_large: 'Przesłane dane są zbyt duże.',
  bad_request: 'Nieprawidłowe żądanie.',
  internal: 'Błąd serwera. Spróbuj ponownie za chwilę.',
  unsupported_media_type: 'Nieobsługiwany format danych.',
  in_use: 'Ten przepis jest użyty w planie. Najpierw usuń go z planu.',
  conflict: 'Nie można wykonać operacji: element już istnieje lub tydzień nie jest pusty.',
  last_admin: 'Gospodarstwo musi mieć co najmniej jednego administratora.',
  cookie_not_stored:
    'Przeglądarka nie zapisała sesji. Jeśli otwierasz aplikację przez zwykłe HTTP, ustaw MEALPLAN_COOKIE_SECURE=false lub użyj HTTPS (np. tailscale serve).',
}

export function errorText(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.code === 'rate_limited') {
      return e.retryAfter
        ? `Zbyt wiele prób. Spróbuj ponownie za ${e.retryAfter} s.`
        : 'Zbyt wiele prób. Spróbuj ponownie za chwilę.'
    }
    return errors[e.code] ?? e.message
  }
  return 'Coś poszło nie tak. Spróbuj ponownie.'
}
