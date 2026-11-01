# Architektura

Mealplan to jeden plik binarny Go z osadzonym frontendem Vue. Dane trzyma w SQLite (tryb `selfhost`) lub Postgres (tryb `hosted`, Etap 5).

## Komponenty (stan po Etapie 2)

```mermaid
flowchart LR
    subgraph Klient
        SPA[Vue 3 SPA + Vuetify]
    end
    subgraph Serwer["Plik binarny Go"]
        SPAH[spa: osadzony frontend]
        API[httpapi: /api/v1]
        AUTH[auth: hasła, sesje, zaproszenia, limiter]
        DOM[recipe, plan, shopping, units: logika domenowa]
        STORE[store: repozytoria i migracje]
    end
    DB[(SQLite lub Postgres)]
    SPA --> SPAH
    SPA --> API
    API --> AUTH --> STORE --> DB
    API --> DOM --> STORE
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
    HOUSEHOLD ||--o{ RECIPE : zawiera
    RECIPE ||--o{ INGREDIENT : sklada_sie
    RECIPE ||--o{ STEP : ma
    RECIPE ||--o{ RECIPE_TAG : oznaczony
    HOUSEHOLD ||--o{ PLAN_ENTRY : planuje
    RECIPE |o--o{ PLAN_ENTRY : uzywany
    APP_USER |o--o{ PLAN_ENTRY : gotuje
    HOUSEHOLD ||--o{ SHOPPING_ITEM : kupuje
    MEMBERSHIP {
        text role "admin lub member"
    }
    RECIPE {
        text source_url "http lub https, puste dozwolone"
        text image_path "zarezerwowane na Etap 3 i 4"
    }
    INGREDIENT {
        text normalized_name
        double quantity "NULL = bez ilosci"
    }
    PLAN_ENTRY {
        text date "YYYY-MM-DD"
        text slot "breakfast lunch dinner snack"
        text recipe_id "albo recipe_id, albo free_text"
        text free_text
        int servings
    }
    SHOPPING_ITEM {
        text week "poniedzialek YYYY-MM-DD"
        text normalized_name
        int checked
        int manual "1 = dodana recznie"
    }
```

Tabela użytkowników nazywa się `app_user`, bo `user` jest słowem zastrzeżonym w Postgres. Czasy są zapisane jako unix seconds (`BIGINT`). Każda tabela domenowa ma `household_id` (`ingredient`, `step` i `recipe_tag` dziedziczą go przez `recipe`).

Rozstrzygnięcia Etapu 2:
- Tagi są w osobnej tabeli `recipe_tag`, żeby statystyki po tagach (Etap 4) były zwykłym `GROUP BY`.
- `shopping_item.normalized_name` służy do dopasowania pozycji przy regeneracji listy.
- Tydzień jest identyfikowany poniedziałkiem w formacie `YYYY-MM-DD` (inna data w ścieżce daje `422 invalid_input`).
- `plan_entry.recipe_id` ma `ON DELETE RESTRICT`: przepis użyty w planie nie da się usunąć (`409 in_use`). `cook_user_id` ma `ON DELETE SET NULL`.
- Wpis planu ma dokładnie jedno z: przepis albo tekst wolny (`CHECK`).

## Generowanie listy zakupów

```mermaid
flowchart TD
    A[POST /shopping/generate week, opcjonalnie from i to] --> B{from i to podane?}
    B -- nie --> C[zakres = cały tydzień]
    B -- tak --> D[zakres = from..to w obrębie tygodnia]
    C --> E[wpisy planu z zakresu]
    D --> E
    E --> F[skalowanie składników: porcje wpisu / porcje bazowe]
    F --> G[sumowanie po znormalizowanej nazwie i jednostce]
    G --> H[odejmowanie zapasów: pusta lista do Etapu 3]
    H --> I[nowe pozycje wygenerowane]
    I --> J{pozycja o tym samym kluczu istniała?}
    J -- nie --> K[nowe id, checked = false]
    J -- tak --> L[zachowaj id]
    L --> M{ilość bez zmian?}
    M -- tak --> N[zachowaj checked]
    M -- nie --> O[checked = false]
    K --> P[zastąp wygenerowane pozycje tygodnia]
    N --> P
    O --> P
    P --> Q[pozycje ręczne zostają nietknięte]
```

Zasady: pozycje wygenerowane są tylko do odhaczania, edytować i usuwać można ręczne. Regeneracja zawsze zastępuje wszystkie wygenerowane pozycje całego tygodnia, więc generowanie z zakresem dni (`from`, `to`) zostawia po sobie wyłącznie pozycje z tego zakresu. Odejmowanie zapasów jest podpięte w `shopping.Build`, ale w Etapie 2 dostaje pustą listę (aktywne od Etapu 3).

## Usunięcie członka gospodarstwa

```mermaid
sequenceDiagram
    actor A as Administrator
    participant S as API
    participant DB as Baza (jedna transakcja)
    A->>S: DELETE /household/members/{id}
    S->>DB: zablokuj administratorów gospodarstwa
    S->>DB: odczytaj rolę członka
    alt to ostatni administrator
        S-->>A: 409 last_admin
    else można usunąć
        S->>DB: plan_entry.cook_user_id = NULL (to gospodarstwo)
        S->>DB: usuń sesje użytkownika w tym gospodarstwie
        S->>DB: usuń członkostwo
        S-->>A: 204
    end
```

Konto użytkownika (`app_user`) zostaje; kasowane są tylko członkostwo i sesje w tym gospodarstwie. Ta sama blokada administratorów chroni zmianę roli (`PATCH`): ostatniego administratora nie można zdegradować.

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
