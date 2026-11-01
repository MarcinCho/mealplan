# Architektura

Mealplan to jeden plik binarny Go z osadzonym frontendem Vue. Dane trzyma w SQLite (tryb `selfhost`) lub Postgres (tryb `hosted`, Etap 5).

## Komponenty (stan po Etapie 1)

```mermaid
flowchart LR
    subgraph Klient
        SPA[Vue 3 SPA + Vuetify]
    end
    subgraph Serwer["Plik binarny Go"]
        SPAH[spa: osadzony frontend]
        API[httpapi: /api/v1]
        AUTH[auth: hasła, sesje, zaproszenia, limiter]
        STORE[store: repozytoria i migracje]
    end
    DB[(SQLite lub Postgres)]
    SPA --> SPAH
    SPA --> API
    API --> AUTH --> STORE --> DB
```

## Model danych

```mermaid
erDiagram
    HOUSEHOLD ||--o{ MEMBERSHIP : ma
    APP_USER ||--o{ MEMBERSHIP : nalezy
    HOUSEHOLD ||--o{ INVITE : wystawia
    APP_USER ||--o{ SESSION : posiada
    HOUSEHOLD ||--o{ SESSION : aktywne
    HOUSEHOLD ||--o{ AUDIT_EVENT : loguje
    MEMBERSHIP {
        text role "admin lub member"
    }
```

Tabela użytkowników nazywa się `app_user`, bo `user` jest słowem zastrzeżonym w Postgres. Czasy są zapisane jako unix seconds (`BIGINT`). Każda przyszła tabela domenowa dostanie `household_id`.

## Pierwsze uruchomienie i zaproszenia

```mermaid
sequenceDiagram
    actor A as Pierwszy użytkownik
    participant W as SPA
    participant S as API
    A->>W: otwiera aplikację
    W->>S: GET /auth/status
    S-->>W: setupRequired = true
    A->>W: formularz konfiguracji
    W->>S: POST /auth/setup
    S-->>W: sesja (admin)
    A->>W: tworzy zaproszenie
    W->>S: POST /auth/invites
    S-->>W: link /invite/{token}
```

## Cykl życia sesji

```mermaid
stateDiagram-v2
    [*] --> Aktywna: login / setup / przyjęcie zaproszenia
    Aktywna --> Aktywna: żądanie (odświeża last_seen)
    Aktywna --> Wygasła: bezczynność > MEALPLAN_SESSION_IDLE
    Aktywna --> Wygasła: wiek > MEALPLAN_SESSION_MAX
    Aktywna --> Zakończona: wylogowanie
    Wygasła --> [*]
    Zakończona --> [*]
```

## Przepływ żądania

Żądanie przechodzi przez nagłówki bezpieczeństwa, router, a dla tras prywatnych przez uwierzytelnienie sesji i (dla metod modyfikujących) sprawdzenie CSRF. Pakiety domenowe nie znają HTTP, a `httpapi` nie zna SQL.
