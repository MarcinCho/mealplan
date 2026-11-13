# Architektura

Mealplan to jeden plik binarny Go z osadzonym frontendem Vue. Dane trzyma w SQLite (tryb `selfhost`) albo w Postgresie. Migracje są przenośne na oba silniki, a testy warstwy `store` przechodzą na obu.

## Komponenty

```mermaid
flowchart LR
    subgraph Klient
        SPA[Vue 3 SPA + Vuetify + Pinia]
    end
    subgraph Serwer["Plik binarny Go"]
        SPAH[spa: osadzony frontend]
        API[httpapi: /api/v1]
        AUTH[auth: hasła, sesje, zaproszenia, limiter]
        DOM[recipe, plan, shopping, units]
        FOOD[product, nutrition, pantry, mealprep]
        IMP[importer: link i tekst na szkic]
        AST[assistant: OpenRouter]
        STORE[store: repozytoria i migracje]
    end
    DB[(SQLite lub Postgres)]
    OR[(openrouter.ai)]
    SPA --> SPAH
    SPA --> API
    API --> AUTH --> STORE --> DB
    API --> DOM --> STORE
    API --> FOOD --> STORE
    API --> IMP
    API --> AST --> STORE
    AST -. tylko z kluczem .-> OR
```

- `importer` niczego nie zapisuje: zwraca szkic, który użytkownik poprawia i zapisuje w edytorze ([Import](import.md)).
- `product` to baza produktów gospodarstwa z aliasami. Z niej korzystają `nutrition` (kalorie), `pantry` (spiżarnia) i `shopping` (odejmowanie zapasów). Startowe produkty pochodzą z `internal/nutrition/seed/products.csv` i trafiają do gospodarstwa przy pierwszym użyciu.
- `assistant` łączy się wyłącznie z `https://openrouter.ai/api/v1/chat/completions` i tylko wtedy, gdy administrator ustawił klucz. Zwraca propozycje; zapis robi zawsze osobne żądanie po zatwierdzeniu ([Bezpieczeństwo](security.md)).

## Model danych

Tabele kont i przepisów:

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
    HOUSEHOLD {
        int portions_per_meal "domyslnie 2"
        int products_seeded
        text openrouter_key "nigdy nie wraca przez API"
        text openrouter_model
    }
    MEMBERSHIP {
        text role "admin lub member"
    }
    INGREDIENT {
        text normalized_name
        double quantity "NULL = bez ilosci"
        text product_id "zapamietany produkt"
    }
    PLAN_ENTRY {
        text date "YYYY-MM-DD"
        text slot "breakfast lunch dinner snack"
        text kind "meal prep out"
        int servings
        int done
    }
    SHOPPING_ITEM {
        text week "poniedzialek YYYY-MM-DD"
        text normalized_name
        int checked
        int manual "1 = dodana recznie"
    }
```

Tabele produktów, spiżarni i meal prepu:

```mermaid
erDiagram
    HOUSEHOLD ||--o{ PRODUCT : ma
    PRODUCT ||--o{ PRODUCT_ALIAS : nazywany
    PRODUCT |o--o{ INGREDIENT : dopasowany
    PRODUCT ||--o{ PANTRY_ITEM : w_spizarni
    PRODUCT ||--o{ PANTRY_MOVEMENT : zdjety
    PRODUCT ||--o{ FOOD_LOG : dopisany
    PRODUCT |o--o{ SHOPPING_ITEM : kupowany
    APP_USER ||--o{ FOOD_LOG : je
    APP_USER ||--o{ NUTRITION_SETTINGS : cel
    PLAN_ENTRY |o--o{ PANTRY_MOVEMENT : zrobione
    HOUSEHOLD ||--o{ PREP_SESSION : gotuje
    PREP_SESSION ||--o{ PREP_DISH : dania
    PREP_SESSION |o--o{ PANTRY_MOVEMENT : ugotowane
    PREP_DISH |o--o{ FREEZER_ITEM : zamrozone
    PREP_DISH |o--o{ PLAN_ENTRY : porcje
    FREEZER_ITEM |o--o{ PLAN_ENTRY : porcje
    PRODUCT {
        text normalized_name "unikalna w gospodarstwie"
        double kcal "na 100 g, tak samo makro"
        double piece_grams
        double density "g/ml"
    }
    PRODUCT_ALIAS {
        text alias "unikalny w gospodarstwie, max 100 na produkt"
    }
    PANTRY_ITEM {
        double quantity "NULL = staly zapas"
        text unit "unikalna z produktem"
        text expires_on
        int staple
    }
    PANTRY_MOVEMENT {
        double amount "ile zdjeto, do oddania przy cofnieciu"
    }
    FOOD_LOG {
        text date
        double quantity
        text unit "g lub szt"
    }
    PREP_DISH {
        int portions
        int written_off
    }
```

Tabela użytkowników nazywa się `app_user`, bo `user` jest słowem zastrzeżonym w Postgresie. Czasy są zapisane jako unix seconds (`BIGINT`). Każda tabela domenowa ma `household_id` (`ingredient`, `step` i `recipe_tag` dziedziczą go przez `recipe`).

Rozstrzygnięcia:
- Tagi są w osobnej tabeli `recipe_tag`, żeby statystyki po tagach były zwykłym `GROUP BY`.
- `shopping_item.normalized_name` służy do dopasowania pozycji przy regeneracji listy.
- Tydzień jest identyfikowany poniedziałkiem w formacie `YYYY-MM-DD` (inna data w ścieżce daje `422 invalid_input`).
- `plan_entry.recipe_id` ma `ON DELETE RESTRICT`: przepisu użytego w planie nie da się usunąć (`409 in_use`). `cook_user_id` ma `ON DELETE SET NULL`.
- Wpis planu ma rodzaj (`kind`): `meal` (dokładnie jedno z: przepis albo tekst wolny), `prep` (porcje z dania sesji albo z zamrażalki) albo `out` (posiłek na mieście z opcjonalnymi wartościami). Pilnuje tego `CHECK`.
- `food_log.product_id` ma `ON DELETE RESTRICT`: produktu z dziennika domowników nie da się usunąć. Scalanie produktów przepina wszystkie odniesienia w jednej transakcji ([Wartości odżywcze](nutrition.md)).
- `pantry_movement` zapamiętuje, co zdjęło „Zrobione” albo „Ugotowane”, więc cofnięcie oddaje dokładnie to samo.

## Generowanie listy zakupów

```mermaid
flowchart TD
    A[POST /shopping/generate week, opcjonalnie from i to] --> B{from i to podane?}
    B -- nie --> C[zakres = cały tydzień]
    B -- tak --> D[zakres = from..to w obrębie tygodnia]
    C --> E[niezrobione wpisy z przepisu z zakresu]
    D --> E
    E --> E2[plus dania sesji meal prepu tego tygodnia]
    E2 --> F[skalowanie składników: porcje wpisu / porcje bazowe]
    F --> G[sumowanie po produkcie albo nazwie i jednostce]
    G --> H[odejmowanie spiżarni: zgodne jednostki, stały zapas pomijany]
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

Zasady: pozycje wygenerowane są tylko do odhaczania, edytować i usuwać można ręczne. Regeneracja zawsze zastępuje wszystkie wygenerowane pozycje całego tygodnia, więc generowanie z zakresem dni (`from`, `to`) zostawia po sobie wyłącznie pozycje z tego zakresu. To, co jest w spiżarni, trafia do sekcji „Masz w domu” zamiast do zakupów; szczegóły w [Spiżarnia](pantry.md). Porcje z meal prepu nie trafiają na listę drugi raz: składniki liczy sesja gotowania ([Meal prep](mealprep.md)).

## Wersje językowe

Interfejs jest po polsku (domyślnie) albo po angielsku. Język wybiera się na stronie logowania i na stronie Konto; zapamiętuje go przeglądarka (`localStorage`, klucz `mealplan.locale`), a zmiana przeładowuje stronę.

- Wszystkie teksty są w `web/src/locales/pl.ts` i `web/src/locales/en.ts`, a komponenty biorą je z `t` w `web/src/messages.ts`. Angielski słownik ma typ polskiego, więc brakujący klucz zatrzymuje build; test `web/src/i18n.test.ts` pilnuje, żeby w angielskich tekstach nie było polskich liter.
- Daty i liczby idą przez `Intl` z tagiem z `web/src/i18n.ts` (`pl-PL` albo `en-GB`). Odmiana porcji i jednostek kuchennych (`łyżka` → `tbsp`) jest w słownikach.
- API zwraca kody błędów; ich treść tłumaczy frontend. Przepisy, nazwy produktów (także startowe) i dokumentacja zostają po polsku.

Nowy język to nowy plik w `web/src/locales/`, wpis w `LOCALES` w `i18n.ts` i komunikaty Vuetify w `plugins/vuetify.ts`.

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
