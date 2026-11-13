# Współpraca

Dziękujemy za zainteresowanie projektem. Uczestnicząc, zgadzasz się przestrzegać [kodeksu postępowania](CODE_OF_CONDUCT.md). Podatności zgłaszaj prywatnie, zgodnie z [SECURITY.md](SECURITY.md).

## Jak zaproponować zmianę

- Błędy: otwórz issue z formularza "Błąd".
- Nowe funkcje: najpierw otwórz issue z formularzem "Propozycja funkcji" i poczekaj na odpowiedź, zanim napiszesz większy kawałek kodu. Oszczędza to pracę, jeśli pomysł nie pasuje do planów projektu.
- Drobne poprawki (literówki, dokumentacja, oczywiste błędy) możesz wysłać od razu jako pull request.

## Środowisko deweloperskie

Potrzebujesz Go 1.26, Node 22 i `make`. Jeśli nie chcesz instalować Go lokalnie, możesz uruchamiać polecenia Go w kontenerze z obrazem `golang:1.26-alpine` (tego samego, który używa `Dockerfile`).

| Polecenie | Co robi |
|---|---|
| `make web` | `npm ci` i `npm run build` w `web/` (buduje interfejs do `web/dist`) |
| `make build` | `make web`, potem binarka `bin/mealplan` (`CGO_ENABLED=0`) |
| `make test` | `go test ./...` i `npm test` w `web/` |
| `make run` | `make build` i uruchomienie z `MEALPLAN_COOKIE_SECURE=false` |

Pułapka: interfejs jest osadzany w binarce z `web/dist`, a katalog ten jest w `.gitignore` (poza `web/dist/.gitkeep`). `web/embed.go` osadza ten katalog (`//go:embed all:dist`), więc musi istnieć. Jeśli kompilacja zgłasza brak plików do osadzenia, a nie chcesz budować interfejsu, wykonaj `mkdir -p web/dist && touch web/dist/.gitkeep`. Binarka zbudowana samym `go build` bez `npm run build` ma pusty interfejs.

## Testy

Tak samo jak w CI (`.github/workflows/ci.yml`):

```bash
# Go na SQLite (domyślnie), z wyścigami
go test ./... -race

# Go na Postgresie (wystarczy pusta baza, każdy test tworzy własny schemat)
MEALPLAN_TEST_DRIVER=postgres \
MEALPLAN_TEST_DSN='postgres://user:pass@localhost:5432/db?sslmode=disable' \
go test ./... -race

# Frontend
cd web
npm ci
npm test          # vitest
npm run build     # vue-tsc --noEmit, potem vite build
```

CI uruchamia testy Go dla obu baz (macierz `sqlite` i `postgres`), testy i budowanie frontendu, `govulncheck` oraz budowanie obrazu Dockera.

## Styl kodu

- `gofmt -l .` nie może nic wypisać, a `go vet ./...` musi przejść (CI to sprawdza).
- SQL musi być przenośny między SQLite i Postgres: znaki `?`, bez składni specyficznej dla jednego silnika.
- Każde zapytanie dotyczące danych użytkownika filtruje po `household_id`. Dodając nowe zapytanie, dodaj test izolacji między dwoma gospodarstwami.
- Zmiana API wymaga aktualizacji `api/openapi.yaml`. Test `TestRoutesMatchOpenAPI` pilnuje zgodności tras ze specyfikacją.
- Napisy interfejsu trzymamy wyłącznie w słownikach `web/src/locales/pl.ts` i `web/src/locales/en.ts`, nie w komponentach. Komponenty biorą je z `t` (`web/src/messages.ts`). Nowy tekst dodaj w obu plikach; angielski słownik ma typ polskiego, więc brak klucza zatrzyma `npm run build`. Daty i liczby formatuj przez `intl` z `web/src/i18n.ts`, nie przez wpisane na stałe `'pl-PL'`.
- Projekt używa Go 1.26 (`go.mod`, CI, Dockerfile). Przy podnoszeniu toolchainu lub zależności zmieniaj te trzy miejsca razem i uruchom `govulncheck ./...` (wersja przypięta w CI).
- Audyt zależności frontendu w CI: `cd web && npm audit --omit=dev --omit=peer --omit=optional --audit-level=high`. Sprawdza tylko zależności dostarczane w produkcie; vite i esbuild to narzędzia czasu budowania i nie trafiają do obrazu.

## Commity

Tematy w formie zbliżonej do Conventional Commits, jak w historii repozytorium: `feat(plan): ...`, `fix(web): ...`, `docs: ...`, `test: ...`, `ci: ...`. Zakres w nawiasie jest opcjonalny. Temat pisz w trybie rozkazującym, po angielsku lub po polsku, spójnie z sąsiednimi commitami.

## Dokumentacja

- Dokumentację piszemy po polsku, diagramy w mermaid (blok ` ```mermaid ` zaczynający się od typu diagramu, np. `flowchart`).
- `go test ./docs/...` sprawdza poprawność bloków mermaid oraz to, że względne odnośniki w plikach Markdown wskazują istniejące pliki.
- Zmieniając zachowanie lub konfigurację, zaktualizuj odpowiednie pliki w `docs/` i wpis w `CHANGELOG.md`.

## Lista kontrolna pull requesta

- [ ] Testy Go (`go test ./... -race`) i frontendu (`npm test`, `npm run build`) przechodzą.
- [ ] `gofmt -l .` jest puste, `go vet ./...` przechodzi.
- [ ] `api/openapi.yaml` jest zaktualizowany, jeśli zmieniło się API.
- [ ] Dokumentacja i `CHANGELOG.md` są zaktualizowane.
- [ ] Migracje (jeśli są) działają na SQLite i Postgresie.
- [ ] Zmiany w interfejsie mają zrzuty ekranu w opisie PR.

```mermaid
flowchart LR
    A[Issue z propozycją] --> B[Zmiana kodu]
    B --> C[Testy Go i web]
    C --> D[Aktualizacja openapi.yaml i docs]
    D --> E[Pull request]
```
