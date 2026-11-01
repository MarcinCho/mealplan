# Changelog

## Unreleased

- Etap 1: konfiguracja, baza z migracjami (SQLite, pod Postgres), logowanie argon2id, sesje, CSRF, limity prób, zaproszenia, API z OpenAPI, powłoka UI Vue + Vuetify, osadzony frontend, Dockerfile, CI.
- Poprawki po przeglądzie końcowym: sprawdzenie konfiguracji przed haszowaniem, ograniczenie współbieżności argon2, odrzucanie zbyt długich loginów, polskie komunikaty błędów i podpowiedź loginu, poprawki dokumentacji (zapraszanie, blokady, README) i CI. Znana, nieużywana luka GO-2026-5932 (`golang.org/x/crypto/openpgp`, brak poprawki, pakiet nie jest importowany).
- Toolchain Go 1.26 i aktualne zależności (m.in. pgx v5.12.0, modernc.org/sqlite v1.61.1, golang.org/x/crypto v0.58.0, golang.org/x/text v0.43.0); `govulncheck` nie zgłasza podatności w używanym kodzie.
