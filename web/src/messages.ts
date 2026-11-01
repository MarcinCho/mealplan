import { ApiError } from './api/client'

export const t = {
  appName: 'Mealplan',
  login: 'Login',
  password: 'Hasło',
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
}

const errors: Record<string, string> = {
  invalid_credentials: 'Nieprawidłowy login lub hasło.',
  rate_limited: 'Zbyt wiele prób. Spróbuj ponownie za chwilę.',
  setup_done: 'Konfiguracja została już wykonana.',
  invite_invalid: t.inviteInvalid,
  login_taken: 'Ten login jest już zajęty.',
  password_length: 'Hasło musi mieć od 12 do 128 znaków.',
  invalid_input: 'Sprawdź poprawność wpisanych danych.',
  forbidden: 'Brak uprawnień do tej operacji.',
  cookie_not_stored:
    'Przeglądarka nie zapisała sesji. Jeśli otwierasz aplikację przez zwykłe HTTP, ustaw MEALPLAN_COOKIE_SECURE=false lub użyj HTTPS (np. tailscale serve).',
}

export function errorText(e: unknown): string {
  if (e instanceof ApiError) return errors[e.code] ?? e.message
  return 'Coś poszło nie tak. Spróbuj ponownie.'
}
