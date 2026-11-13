# Bezpieczeństwo

## Zabezpieczenia

| Obszar | Rozwiązanie |
|---|---|
| Hasła | argon2id (m=64 MiB, t=3, p=2), 12 do 128 znaków, ograniczenie długości przed hashowaniem |
| Sesje | token 256 bit, w bazie tylko hash SHA-256; wygasanie po bezczynności (domyślnie 7 dni) i limit bezwzględny (domyślnie 30 dni) |
| Ciastka | sesyjne: `HttpOnly`, `Secure` (konfigurowalne), `SameSite=Lax`; ciastko CSRF ma te same `Secure` i `SameSite`, ale bez `HttpOnly`, bo czyta je frontend |
| CSRF | double-submit: ciastko `mealplan_csrf` + nagłówek `X-CSRF-Token` zgodne ze sobą i z tokenem sesji (porównanie w stałym czasie) |
| Trasy publiczne | tylko `Content-Type: application/json`, co blokuje formularze z innych witryn |
| Logowanie | limit na login (blokada od 5. kolejnej porażki) i na IP (od 30.), blokada 1 s podwajana do 15 min; identyczny błąd dla złego loginu i hasła; hashowanie atrapy dla nieistniejącego loginu wyrównuje czas odpowiedzi |
| Zaproszenia | jednorazowe, ważne 72 godziny (stała w `cmd/mealplan/main.go`, nie zmienna środowiskowa), token hashowany, zużycie atomowe w transakcji |
| Izolacja gospodarstw | `household_id` w zapytaniach, testy między dwoma gospodarstwami; obejmuje przepisy, plan i listę zakupów (obcy zasób zwraca `404 not_found`); wpis planu może wskazać tylko przepis i kucharza z własnego gospodarstwa |
| Integralność | włączone klucze obce (`foreign_keys`) w SQLite; przepis użyty w planie nie da się usunąć (`ON DELETE RESTRICT`, `409 in_use`) |
| Walidacja wejścia | `sourceUrl` tylko `http` lub `https` z hostem (odrzucone `javascript:`, `data:`); limity długości nazw, liczby składników, ilości i porcji |
| Członkowie | usunięcie członka kasuje jego sesje w gospodarstwie i czyści go z roli kucharza; ostatniego administratora nie można zdegradować ani usunąć (`409 last_admin`, blokada administratorów w transakcji) |
| Nagłówki | CSP, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY`, HSTS przy `Secure`, `Cache-Control: no-store` dla `/api/` |
| Audyt | tabela `audit_event` (zdarzenia `setup`, `login`, `login_failed`, `invite_created`, `invite_revoked`, `invite_accepted`, `role_changed`, `member_removed`) |

## Import z adresu URL (SSRF)

Import przepisu z linku sprawia, że serwer łączy się z adresem podanym przez użytkownika. Szczegóły działania: [import.md](import.md).

| Obszar | Rozwiązanie |
|---|---|
| Blokowane zakresy | loopback (`127.0.0.0/8`, `::1`), prywatne (`10/8`, `172.16/12`, `192.168/16`, `fc00::/7`), link-local (`169.254/16`, `fe80::/10`), multicast, adres nieokreślony, `0.0.0.0/8`, `100.64.0.0/10` (CGNAT i adresy tailnetu Tailscale), `198.18.0.0/15` (testy wydajności), `240.0.0.0/4` (zarezerwowane, w tym `255.255.255.255`), `192.0.0.0/24` (protokoły IETF) oraz zakresy IPv6 osadzające IPv4 lub przestarzałe: `64:ff9b::/96` i `64:ff9b:1::/48` (NAT64), `2002::/16` (6to4), `::/96` (zgodne z IPv4), `fec0::/10` (site-local) i `2001::/32` (Teredo); adresy IPv4 zapisane jako IPv6 (`::ffff:a.b.c.d`) są sprawdzane jako IPv4 |
| Moment kontroli | w dialerze (`net.Dialer.Control`), po rozwiązaniu DNS i tuż przed połączeniem z konkretnym IP; dzięki temu zabezpieczone są też każde przekierowanie i rebinding DNS (nazwa rozwiązująca się najpierw na adres publiczny, potem na prywatny) |
| Przekierowania | najwyżej 5; przekierowanie na nie-`http(s)`, adres z danymi logowania lub inny port niż 80/443 kończy się błędem |
| Schematy i porty | tylko `http` i `https`, porty 80 i 443, bez `user:pass@` |
| Proxy | brak: proxy z zmiennych środowiskowych (`HTTP_PROXY` itd.) jest wyłączone, żeby nie obejść kontroli adresów |
| Limity pobierania | 2 MB treści, 10 s na całe żądanie, tylko typy HTML; połączenia bez keep-alive |
| Limit częstotliwości | 10 importów z linku na minutę na użytkownika (`429 rate_limited`), żeby serwer nie służył do skanowania sieci; licznik jest w pamięci procesu, jak limiter logowania |
| Treść błędów | odpowiedź zawiera tylko kod (`import_blocked`, `import_failed`, `import_timeout`); przyczyna błędu sieci jest odrzucana, bo komunikat z dialera zdradzałby, które wewnętrzne hosty istnieją |
| Wynik | szkic to dane: tekst jest zawsze escapowany przy wyświetlaniu (interpolacja Vue, bez `v-html`), a surowy HTML pobranej strony nigdy nie jest renderowany; `sourceUrl` przechodzi tę samą walidację `http`/`https` co zwykły przepis |
| Brak zapisu | endpointy importu niczego nie zapisują; przepis powstaje dopiero z zatwierdzonego podglądu |

## Asystent produktów (OpenRouter)

| Obszar | Rozwiązanie |
|---|---|
| Klucz | opcjonalny klucz OpenRouter gospodarstwa jest przechowywany jawnym tekstem w wierszu gospodarstwa w bazie (serwer musi go wysłać dalej, więc nie da się go zahaszować). Kopie zapasowe bazy go zawierają, więc traktuj je jak sekret |
| Dostęp | API nigdy nie zwraca klucza, tylko `keyHint` (ostatnie 4 znaki); ustawić i usunąć go może tylko administrator |
| Dane wysyłane | do OpenRoutera trafiają nazwy składników z ilością i jednostką, nazwy produktów i aliasy oraz (przy szukaniu duplikatów) wartości na 100 g. Nie są wysyłane: hasła, loginy, plan, dziennik jedzenia, spiżarnia ani adresy e-mail |
| Model | domyślnie `google/gemini-3.1-flash-lite` (`DefaultModel` w `internal/assistant/openrouter.go`); administrator zmienia go w ustawieniach |
| Limit | endpointy modelu dzielą limit importu: 10 zapytań na minutę na użytkownika (`429 rate_limited`) |
| Błędy | odpowiedź niesie kod (`assistant_bad_key`, `assistant_no_credit`, `assistant_bad_model`, `assistant_unavailable`) i oczyszczony komunikat (status oraz skrócony opis błędu OpenRoutera, bez klucza); klucz nie trafia do logów ani komunikatów |
| Skutki | model niczego nie zapisuje ani nie scala: propozycje są danymi, a zmiana następuje po zatwierdzeniu przez użytkownika |

## Znane ograniczenia

- **CSP dla stylów** dopuszcza `unsafe-inline`, bo Vuetify wstrzykuje arkusz motywu w czasie działania. Skrypty mają ścisłe `script-src 'self'`. Docelowo: nonce.
- **Limiter prób jest w pamięci procesu.** Przy jednej instancji (`selfhost`) wystarcza. W trybie `hosted` z wieloma instancjami trzeba go przenieść do bazy (Etap 5). Restart procesu zeruje liczniki.
- **Adres IP klienta** to `RemoteAddr`. Za reverse proxy (np. `tailscale serve`) wszystkie żądania mają ten sam klucz IP. W Dockerze z `-p 127.0.0.1:8080:8080` `RemoteAddr` to adres bramy mostu Dockera (np. 172.17.0.1), a nie 127.0.0.1. Wpisy `ip=` w dzienniku audytu nie są tam więc miarodajne. Zaufane proxy i `X-Forwarded-For` dojdą w Etapie 5.
- **Blokada może być nadużyta w obrębie tailnetu.** Uporczywy atakujący w tailnecie może utrzymywać blokadę znanego loginu (pojedyncza blokada ma maksymalnie 15 minut, ale można ją odnawiać kolejnymi porażkami). Około 40 porażek wystarcza, by zablokować wspólny kubełek IP, przez co logowanie wszystkich jest wstrzymane. Istniejące sesje nadal działają.
- **Argon2 jest ograniczony współbieżnie.** Haszowanie i weryfikacja haseł korzystają z semafora o rozmiarze min(max(CPU, 1), 4), więc lawina logowań nie wyczerpie pamięci. `Setup` sprawdza, czy konfiguracja już została wykonana, przed haszowaniem hasła (transakcyjna kontrola w bazie pozostaje zabezpieczeniem ostatecznym).
- Blokada loginu pozwala atakującemu chwilowo zablokować znane konto. Blokada jest ograniczona do 15 minut.

## Zgłaszanie podatności

Opisz problem w prywatnym zgłoszeniu (GitHub Security Advisories) zamiast publicznego issue.
