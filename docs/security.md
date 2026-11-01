# Bezpieczeństwo

## Zaimplementowane w Etapie 1

| Obszar | Rozwiązanie |
|---|---|
| Hasła | argon2id (m=64 MiB, t=3, p=2), 12 do 128 znaków, ograniczenie długości przed hashowaniem |
| Sesje | token 256 bit, w bazie tylko hash SHA-256; wygasanie po bezczynności (domyślnie 7 dni) i limit bezwzględny (domyślnie 30 dni) |
| Ciastka | sesyjne: `HttpOnly`, `Secure` (konfigurowalne), `SameSite=Lax`; ciastko CSRF ma te same `Secure` i `SameSite`, ale bez `HttpOnly`, bo czyta je frontend |
| CSRF | double-submit: ciastko `mealplan_csrf` + nagłówek `X-CSRF-Token` zgodne ze sobą i z tokenem sesji (porównanie w stałym czasie) |
| Trasy publiczne | tylko `Content-Type: application/json`, co blokuje formularze z innych witryn |
| Logowanie | limit na login (blokada od 5. kolejnej porażki) i na IP (od 30.), blokada 1 s podwajana do 15 min; identyczny błąd dla złego loginu i hasła; hashowanie atrapy dla nieistniejącego loginu wyrównuje czas odpowiedzi |
| Zaproszenia | jednorazowe, ważne 72 godziny (stała w `cmd/mealplan/main.go`, nie zmienna środowiskowa), token hashowany, zużycie atomowe w transakcji |
| Izolacja gospodarstw | `household_id` w zapytaniach, testy między dwoma gospodarstwami |
| Nagłówki | CSP, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY`, HSTS przy `Secure`, `Cache-Control: no-store` dla `/api/` |
| Audyt | tabela `audit_event` (zdarzenia `setup`, `login`, `login_failed`, `invite_created`, `invite_revoked`, `invite_accepted`) |

## Znane ograniczenia

- **CSP dla stylów** dopuszcza `unsafe-inline`, bo Vuetify wstrzykuje arkusz motywu w czasie działania. Skrypty mają ścisłe `script-src 'self'`. Docelowo: nonce.
- **Limiter prób jest w pamięci procesu.** Przy jednej instancji (`selfhost`) wystarcza. W trybie `hosted` z wieloma instancjami trzeba go przenieść do bazy (Etap 5). Restart procesu zeruje liczniki.
- **Adres IP klienta** to `RemoteAddr`. Za reverse proxy (np. `tailscale serve`) wszystkie żądania mają adres 127.0.0.1, dlatego limit IP jest hojny, a główną ochroną jest limit na login. Zaufane proxy i `X-Forwarded-For` dojdą w Etapie 5.
- Blokada loginu pozwala atakującemu chwilowo zablokować znane konto. Blokada jest ograniczona do 15 minut.

## Zgłaszanie podatności

Opisz problem w prywatnym zgłoszeniu (GitHub Security Advisories) zamiast publicznego issue.
