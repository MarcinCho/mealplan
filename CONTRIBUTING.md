# Współpraca

1. Backend: `go test ./... -race`. Frontend: `cd web && npm test`.
2. Testy bazy uruchamiają się domyślnie na SQLite. Aby przetestować Postgres: `MEALPLAN_TEST_DRIVER=postgres MEALPLAN_TEST_DSN='postgres://user:pass@localhost:5432/db?sslmode=disable' go test ./...`. Każdy test tworzy własny schemat, więc wystarczy pusta baza.
3. Zmiana API wymaga aktualizacji `api/openapi.yaml` (test to egzekwuje).
4. SQL musi być przenośny między SQLite i Postgres (znaki `?`, bez składni specyficznej dla jednego silnika).
5. Diagramy w dokumentacji piszemy w mermaid.
6. Commity w formie `feat:`, `fix:`, `docs:`, `test:`.
7. Projekt używa Go 1.26 (`go.mod`, CI, Dockerfile) i aktualnych zależności; przy podnoszeniu toolchainu lub zależności zmieniaj te trzy miejsca razem i uruchom `govulncheck ./...` (wersja przypięta w CI).
8. Audyt zależności frontendu w CI: `cd web && npm audit --omit=dev --omit=peer --omit=optional --audit-level=high`. Sprawdza tylko zależności dostarczane w produkcie; vite i esbuild to narzędzia czasu budowania i nie trafiają do obrazu.

```mermaid
flowchart LR
    A[Zmiana kodu] --> B[Testy Go i web]
    B --> C[Aktualizacja openapi.yaml i docs]
    C --> D[Pull request]
```
