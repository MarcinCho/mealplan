# Changelog

## Unreleased

- Etap 1: konfiguracja, baza z migracjami (SQLite, pod Postgres), logowanie argon2id, sesje, CSRF, limity prób, zaproszenia, API z OpenAPI, powłoka UI Vue + Vuetify, osadzony frontend, Dockerfile, CI.
- Toolchain Go 1.26 i aktualne zależności (m.in. pgx v5.12.0, modernc.org/sqlite v1.61.1, golang.org/x/crypto v0.58.0, golang.org/x/text v0.43.0); `govulncheck` nie zgłasza podatności w używanym kodzie.
