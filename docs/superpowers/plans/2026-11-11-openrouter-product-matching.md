# Dopasowanie produktów przez OpenRouter — plan wdrożenia

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Opcjonalny klucz OpenRouter w gospodarstwie. Model podpowiada produkty dla składników (import, „Sprawdź przepisy”) i duplikaty w bazie produktów („Sprawdź produkty”). Wszystko czeka na zatwierdzenie, a zatwierdzona nazwa trafia do aliasów.

**Architecture:** Nowy pakiet `internal/assistant` zawiera klienta OpenRouter (interfejs + fałszywka w testach), instrukcje dla modelu, walidację odpowiedzi i deterministyczne wybieranie kandydatów. Operacje bez modelu trafiają do `store` i `product.Service`: alias, przypisanie składników, użycie produktu, scalanie. Partiami steruje przeglądarka (jedno wywołanie API = jedna partia), a propozycje nie są nigdzie zapisywane.

**Tech Stack:** Go (net/http, database/sql, SQLite + Postgres), Vue 3 + Vuetify 3 + Pinia, Vitest, OpenAPI w `api/openapi.yaml`.

**Spec:** `docs/superpowers/specs/2026-11-11-openrouter-product-matching-design.md`

## Global Constraints

- Gałąź `feat/openrouter-products`. Commity jako `git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit`, z trailerem `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Narzędzia: na hoście nie ma Go ani Node, więc używaj `.superpowers/bin/dkgo go …` i `.superpowers/bin/dknode npm …|npx …` (uruchamia w `web/`). Po `npm run build` wykonaj `touch web/dist/.gitkeep`.
- Migracje muszą być przenośne na SQLite i Postgres. Testy store przechodzą na obu silnikach: domyślnie SQLite, Postgres przez `MEALPLAN_TEST_DRIVER=postgres` w CI.
- Każda nowa trasa ma wpis w `api/openapi.yaml`, inaczej padnie `TestRoutesMatchOpenAPI`.
- Interfejs po polsku, teksty w `web/src/messages.ts`, mobile-first. Nazwy produktów w mianowniku l. poj., poza rzeczownikami bez liczby pojedynczej („płatki owsiane”).
- **Odmiany to nie duplikaty:** czerwona cebula i cebula, papryka czerwona i żółta to różne produkty. Scalane są tylko te same rzeczy zapisane inaczej: liczba mnoga, przypadek, szyk, literówka, zdrobnienie.
- Nic nie zapisuje się bez kliknięcia „Zatwierdź”, „Scal” albo „Zapisz”.
- Klucz nigdy nie wychodzi przez API ani do logów. API zwraca tylko `keyHint` = ostatnie 4 znaki.
- Host modelu jest stały: `https://openrouter.ai/api/v1/chat/completions`. Żaden test nie łączy się z prawdziwym OpenRouterem.
- Serwer ma `WriteTimeout: 30s` (`cmd/mealplan/main.go`). Handlery modelu przedłużają termin zapisu przez `http.NewResponseController(w).SetWriteDeadline(time.Now().Add(75*time.Second))`, a limit jednego wywołania modelu to **60 s**.
- Partie: do **25 składników** na wywołanie dopasowania i do **80 produktów** na wywołanie duplikatów.
- Zakresy liczb w propozycji nowego produktu są takie jak w `product.Service`: kcal 0–900, makro 0–100 g na 100 g, `pieceGrams` 0–5000.
- Testy uruchamiaj komendami `.superpowers/bin/dkgo go test ./...` i `.superpowers/bin/dknode npx vitest run`. Przed commitem frontu dodatkowo `.superpowers/bin/dknode npm run build`.

## Review Focus

1. **Scalanie produktu, którego pozycja spiżarni ma tę samą jednostkę co pozycja produktu `keep`**, przy `UNIQUE (household_id, product_id, unit)`. Oczekiwane: jedna pozycja z sumą, bez błędu ograniczenia. Test w Task 3.
2. **Model zwraca id produktu spoza przesłanej listy, z innego gospodarstwa albo zmyślone.** Oczekiwane: propozycja zamienia się w „brak”, nigdy w przypisanie. Test w Task 4.
3. **Alias, który już należy do innego produktu.** Oczekiwane: przypisanie składnika się zapisuje, alias nie, a wynik ma `aliasConflict` z nazwą tamtego produktu. Test w Task 2.
4. **Członek bez roli admina próbuje ustawić lub usunąć klucz.** Oczekiwane: 403. `GET` pokazuje mu tylko `configured`, `keyHint` i `model`. Test w Task 5.
5. **OpenRouter zwraca 401, 402 albo 5xx, albo nie odpowiada w czasie.** Oczekiwane: edytor przepisu działa dalej i pokazuje jedną informację, że podpowiedzi są niedostępne. API zwraca 502 `assistant_unavailable` z polskim komunikatem, w którym nie ma klucza. Testy w Task 4 (mapowanie błędów) i Task 7 (edytor).

---

### Task 1: Schemat, osobna cebula czerwona

**Files:**
- Create: `internal/store/migrations/0008_assistant.sql`
- Modify: `internal/nutrition/seed/products.csv` (wiersz 99 `cebula;…` i nowy wiersz)
- Test: `internal/store/migrate_test.go`, `internal/nutrition/seed/*_test.go` (istniejący test seeda)

**Interfaces:**
- Produces: kolumny `household.openrouter_key TEXT NOT NULL DEFAULT ''` i `household.openrouter_model TEXT NOT NULL DEFAULT ''`. Seed ma produkt `cebula czerwona`.

- [ ] **Step 1: Napisz failing testy**
  - `TestMigration0008SplitsRedOnion` w `migrate_test.go`. Migruj do `0007_mealprep` przez `MigrateTo` (eksport w `export_test.go`), wstaw gospodarstwo z produktem `cebula` z aliasami `cebula czerwona`, `cebuli czerwonej`, `cebula biała`, migruj do końca. Oczekuj, że `cebula` ma alias `cebula biała` i nie ma dwóch czerwonych. Istnieje produkt o `normalized_name = 'cebula czerwona'` z aliasem `czerwona cebula`.
  - Drugie gospodarstwo ma już produkt `cebula czerwona`. Po migracji nie ma duplikatu (dokładnie 1 taki produkt).
  - W teście seeda: `cebula` nie ma aliasu zawierającego „czerwon”, a `cebula czerwona` istnieje i ma alias `czerwona cebula`.
- [ ] **Step 2: Uruchom**, oczekiwany FAIL.
- [ ] **Step 3: Zaimplementuj**
  - CSV: z wiersza `cebula` usuń `cebula czerwona|cebuli czerwonej`. Dodaj wiersz `cebula czerwona;czerwona cebula|czerwonej cebuli|cebuli czerwonej|czerwoną cebulę|cebulka czerwona|czerwona cebulka|czerwonej cebulki;40;1.1;9.3;0.1;110;`. Wartości to USDA red onion; jeśli w repo jest skrypt `internal/nutrition/seed/gen`, sprawdź liczby z jego źródłem.
  - Migracja:
    1. `ALTER TABLE household ADD COLUMN …` dla dwóch kolumn.
    2. `DELETE FROM product_alias WHERE alias IN ('cebula czerwona','cebuli czerwonej') AND product_id IN (SELECT id FROM product WHERE normalized_name='cebula')`.
    3. `INSERT … SELECT` produktu `cebula czerwona` dla każdego gospodarstwa z `products_seeded = 1`, które nie ma takiej nazwy ani aliasu. id = `'seed-redonion-' || household.id`, znaczniki czasu jak w innych migracjach (BIGINT). Potem aliasy z CSV, z pominięciem tych już zajętych w gospodarstwie (`NOT EXISTS`).
- [ ] **Step 4: Uruchom** `.superpowers/bin/dkgo go test ./internal/store/... ./internal/nutrition/...`, oczekiwany PASS.
- [ ] **Step 5: Commit** `feat(products): red onion is its own product`

### Task 2: Store i serwis — ustawienia, alias, przypisanie, użycie

**Files:**
- Modify: `internal/store/household.go`, `internal/store/products.go`, `internal/store/recipes.go`
- Modify: `internal/product/service.go`
- Test: `internal/store/household_test.go`, `internal/store/products_test.go`, `internal/product/service_test.go`

**Interfaces:**
- Produces (store):
  - `func (s *Store) AssistantSettings(ctx, householdID string) (key, model string, err error)`
  - `func (s *Store) SetAssistantSettings(ctx, householdID, key, model string) error` (nieznane gospodarstwo → `ErrNotFound`)
  - `func (s *Store) AssignIngredients(ctx, householdID, productID, normalizedName string, recipeIDs []string, alias string) (updated int, aliasOwner string, err error)`. W jednej transakcji ustawia `ingredient.product_id = productID` dla wierszy `normalized_name = normalizedName` w podanych przepisach tego gospodarstwa i dodaje alias, jeśli niepusty i wolny. Jeśli alias ma inny produkt, zwraca jego nazwę w `aliasOwner` i aliasu nie dodaje. Alias równy nazwie lub istniejącemu aliasowi tego produktu jest no-op.
  - `type ProductUsage struct{ Recipes, Pantry, FoodLog, Shopping int }`
  - `func (s *Store) ProductUsage(ctx, householdID, productID string) (ProductUsage, error)`, gdzie `Recipes` = liczba różnych przepisów.
  - `func (s *Store) IngredientNames(ctx, householdID string) ([]IngredientRef, error)` z `type IngredientRef struct{ RecipeID, RecipeTitle, Name, NormalizedName, ProductID string; Quantity *float64; Unit string }`
- Produces (product.Service, z walidacją i sprawdzeniem gospodarstwa; jak istniejące metody przyjmuje `householdID` i wywołuje `ensureSeeded`):
  - `func (s *Service) Assign(ctx, householdID, productID, name string, recipeIDs []string) (AssignResult, error)` z `type AssignResult struct{ Updated int; AliasAdded bool; AliasConflict string }`. Alias = `units.NormalizeName(name)`, max 200 znaków. Max 500 recipeIDs. Nieznany produkt → `ErrNotFound`.
  - `func (s *Service) Usage(ctx, householdID, productID string) (store.ProductUsage, error)`
  - Rozszerz `Repo` o nowe metody.

- [ ] **Step 1: Failing testy store**
  - `TestAssistantSettingsRoundTrip`: domyślnie `"", ""`; po `Set` odczyt zwraca te same wartości; inne gospodarstwo dostaje `ErrNotFound` przy `Set`.
  - `TestAssignIngredientsAddsAlias`: przepisy r1 i r2 mają składnik „czerwona cebula”. `Assign(..., "pRed", "czerwona cebula", [r1], "czerwona cebula")` → `updated == 1`, r2 nadal bez produktu, `pRed` ma alias.
  - `TestAssignIngredientsAliasConflict`: alias „cebulka” należy do p1. Przypisanie do p2 → `aliasOwner == "cebula"`, składnik ma p2, a p2 nie ma aliasu.
  - `TestAssignIngredientsOtherHousehold`: recipeID z innego gospodarstwa → `updated == 0`.
  - `TestProductUsage`: produkt w 2 przepisach (3 składniki), 1 pozycji spiżarni, 1 wpisie dziennika → `{2,1,1,0}`.
- [ ] **Step 2: Uruchom**, FAIL.
- [ ] **Step 3: Zaimplementuj** metody store, potem serwisu, wzorując się na `PortionsPerMeal` i `insertAliases`.
- [ ] **Step 4: Testy serwisu:** `TestAssignValidates` (pusta nazwa → `ErrInvalid`; 501 recipeIDs → `ErrInvalid`; obcy produkt → `ErrNotFound`). Uruchom `go test ./internal/store/... ./internal/product/...`, PASS.
- [ ] **Step 5: Commit** `feat(products): assign ingredients to a product and learn the alias`

### Task 3: Scalanie produktów

**Files:**
- Modify: `internal/store/products.go`, `internal/product/service.go`
- Test: `internal/store/products_test.go`, `internal/product/service_test.go`

**Interfaces:**
- Produces:
  - `func (s *Store) MergeProducts(ctx, householdID, keepID string, absorbIDs []string) (MergeResult, error)` z `type MergeResult struct{ Repointed ProductUsage; SkippedAliases []string }`
  - `func (s *Service) Merge(ctx, householdID, keepID string, absorbIDs []string) (store.MergeResult, error)`. Walidacja: 1–20 absorbIDs, `keepID` nie może być wśród nich, wszystkie muszą należeć do gospodarstwa (inaczej `ErrNotFound`).

- [ ] **Step 1: Failing testy store**
  - `TestMergeRepointsEverything`: produkt A (keep) i B. B jest w składniku przepisu, w `food_log`, w `shopping_item`, w `pantry_movement` i ma pozycję spiżarni w `g`. Po scaleniu wszystkie wskazują A, B nie istnieje, a A ma aliasy: nazwę B i aliasy B.
  - `TestMergeSumsPantrySameUnit`: A ma 200 g (termin 2026-12-01), B ma 300 g (termin 2026-11-20) → jedna pozycja A 500 g z terminem 2026-11-20.
  - `TestMergeStapleWins`: B jest stałym zapasem w `szt`, A ma 3 szt → jedna pozycja `Staple=true`, `Quantity=nil`.
  - `TestMergeKeepsOtherUnits`: A ma g, B ma szt → dwie pozycje A.
  - `TestMergeSkipsAliasOwnedByThird`: nazwa B jest już aliasem produktu C. Wtedy `SkippedAliases == ["<nazwa B>"]`, alias zostaje przy C, a scalenie się udaje.
  - Każdy test przechodzi na SQLite i Postgresie.
- [ ] **Step 2: Uruchom**, FAIL.
- [ ] **Step 3: Zaimplementuj** w jednej transakcji, w kolejności:
  1. `UPDATE` w `ingredient`, `food_log`, `shopping_item`, `pantry_movement`;
  2. spiżarnia: dla każdej pozycji B szukaj pozycji A w tej samej jednostce, wtedy suma, `min(termin)` (pusty = brak terminu, przegrywa z datą) i staple OR, a pozycję B usuń; w przeciwnym razie `UPDATE product_id`;
  3. aliasy B przenieś (`UPDATE product_alias SET product_id`);
  4. nazwę B dodaj jako alias A, jeśli jest wolna, w przeciwnym razie trafia do `SkippedAliases`;
  5. `DELETE` produktu B.
  
  `Repointed` zlicz przed zmianami (suma `ProductUsage` wszystkich B).
- [ ] **Step 4: Uruchom** `go test ./internal/store/... ./internal/product/...`, PASS.
- [ ] **Step 5: Commit** `feat(products): merge duplicate products without losing data`

### Task 4: Pakiet `internal/assistant`

**Files:**
- Create: `internal/assistant/openrouter.go` (klient), `prompt.go` (instrukcje), `parse.go` (walidacja), `candidates.go` (kandydaci), `service.go`
- Test: `internal/assistant/*_test.go`, fałszywy klient w `fake_test.go` i serwer `httptest`

**Interfaces:**
- Consumes: `store.AssistantSettings`, `store.IngredientNames`, `product.Service.All(ctx, householdID) ([]domain.Product, error)`.
- Produces:
  - `type Client interface{ Complete(ctx context.Context, key, model string, system, user string, schema json.RawMessage) (string, error) }`
  - `func NewOpenRouter(hc *http.Client) Client`. Wysyła POST na stały URL z `Authorization: Bearer <key>`, `response_format: {type:"json_schema", json_schema:{name, strict:true, schema}}` i `temperature: 0`. Odpowiedź czyta z limitem 1 MiB, zwraca `choices[0].message.content`. Nagłówki `HTTP-Referer` i `X-Title: Mealplan`.
  - Błędy: `var ErrNotConfigured`, `ErrUnavailable` (opakowuje status/timeout), `ErrBadKey` (401), `ErrNoCredit` (402). W komunikacie błędu nigdy nie ma klucza.
  - `const DefaultModel = "<wybrany>"`. Przed implementacją pobierz `https://openrouter.ai/api/v1/models` (publiczne, bez klucza) i wybierz tani, szybki model z `structured_outputs` w `supported_parameters`. Zapisz wybór i uzasadnienie w komentarzu.
  - `type IngredientQuery struct{ Name string; Quantity *float64; Unit string }`
  - `type NewProduct struct{ Name string; Per100g domain.Macros; PieceGrams, DensityGPerML *float64; Aliases []string }`
  - `type Suggestion struct{ Name string; Kind string /* "existing" | "new" | "none" */; ProductID string; ProductName string; New *NewProduct; Reason string }`
  - `type DuplicateGroup struct{ KeepID string; AbsorbIDs []string; Reason string }`
  - `func CandidateIngredients(refs []store.IngredientRef, products []domain.Product) []IngredientCandidate` z `type IngredientCandidate struct{ Name, NormalizedName string; CurrentProductID string; RecipeIDs []string; RecipeTitles []string }`. Kandydat to składnik bez produktu, którego nazwa nie pasuje dokładnie (`nutrition.NewMatcher`), albo składnik z produktem, którego nazwa i aliasy nie zawierają tej znormalizowanej nazwy. Grupowanie po `NormalizedName` i `CurrentProductID`, sortowanie po nazwie.
  - `func CandidateProductBatches(products []domain.Product, max int) [][]string`: posortuj po `units.NormalizeName(Name)` i grupuj po kluczu = pierwsze 4 runy pierwszego słowa. Pakuj całe grupy do partii ≤ `max`; grupy jednoelementowe (bez sąsiada) pomiń. Grupa większa niż `max` dzieli się na kolejne partie.
  - `type Service struct`, `func NewService(repo SettingsRepo, products ProductLister, c Client) *Service`
  - `func (s *Service) Status(ctx, p) (configured bool, keyHint, model string, err error)`
  - `func (s *Service) Configure(ctx, p, key *string, model string) error`. Tylko `RoleAdmin` (inaczej `auth.ErrForbidden`). `key == nil` oznacza „nie zmieniaj”, a `""` oznacza „usuń”. Model pusty oznacza `DefaultModel`. Walidacja: klucz ≤ 200 znaków, bez białych znaków; model ≤ 100 znaków, wzorzec `^[a-z0-9._-]+/[a-z0-9._:-]+$`.
  - `func (s *Service) Test(ctx, p) error`
  - `func (s *Service) MatchIngredients(ctx, p, qs []IngredientQuery) ([]Suggestion, error)`. Najpierw dokładne dopasowanie lokalne (`Kind:"existing"`, `Reason:"exact"`, bez modelu), potem model dla reszty. Maks 25 zapytań (więcej → `ErrInvalid`).
  - `func (s *Service) FindDuplicates(ctx, p, productIDs []string) ([]DuplicateGroup, error)`, maks 80.
  - `func (s *Service) IngredientCandidates(ctx, p) ([]IngredientCandidate, error)` i `func (s *Service) ProductBatches(ctx, p) ([][]string, error)` (z `max = 80`).
- Instrukcja dla modelu (`prompt.go`, po polsku albo po angielsku z polskimi przykładami) musi zawierać: zasadę odmiany vs duplikat z Global Constraints z przykładami (czerwona cebula ≠ cebula, „pomidory” = „pomidor”); zasadę mianownika l. poj.; „używaj wyłącznie id z listy”; „gdy nie wiesz, zwróć none”; wartości odżywcze na 100 g surowego produktu.
- Walidacja (`parse.go`):
  - JSON zgodny ze schematem;
  - `existing` z id spoza przesłanej listy → `none`;
  - `new` z liczbami poza zakresami albo pustą nazwą → `none`;
  - nazwa `new` równa istniejącej nazwie lub aliasowi → `existing` z tym produktem;
  - duplikaty: id spoza partii są odrzucane, `KeepID` nie może być w `AbsorbIDs`, grupa z pustym `AbsorbIDs` jest odrzucana, id w więcej niż jednej grupie zostaje tylko w pierwszej.

- [ ] **Step 1: Failing testy**
  - `TestOpenRouterRequestShape` (httptest; podmień URL przez nieeksportowane pole w teście): nagłówek Bearer, `model`, `response_format.type == "json_schema"`, treść zwrócona z `choices[0]`.
  - `TestOpenRouterErrors`: 401 → `ErrBadKey`, 402 → `ErrNoCredit`, 500 → `ErrUnavailable`, opóźnienie > timeout klienta → `ErrUnavailable`. W żadnym `err.Error()` nie występuje klucz.
  - `TestParseRejectsForeignIDs` (Review Focus 2).
  - `TestParseNewProductOutOfRange` (kcal 1200 → `none`) i `TestParseNewProductThatExists` (→ `existing`).
  - `TestCandidateIngredients`: „czerwona cebula” bez produktu → kandydat; „cebula” bez produktu (dokładne dopasowanie) → nie; „czerwona cebula” przypięta do `cebula` → kandydat z `CurrentProductID`; „cebulka” przypięta do produktu z aliasem „cebulka” → nie. Dwa przepisy z tą samą nazwą → jeden kandydat z 2 `RecipeIDs`.
  - `TestCandidateProductBatches`: `pomidor`, `pomidory`, `pomidorki koktajlowe`, `jajko` przy `max = 80` → jedna partia z trzema pomidorami, bez jajka.
  - `TestMatchIngredientsExactSkipsModel`: fałszywy klient z licznikiem wywołań; jedno dokładne i jedno nieznane → 1 wywołanie z 1 składnikiem.
  - `TestConfigureAdminOnly` i `TestStatusHidesKey` (`keyHint == "abcd"` dla klucza `sk-or-…abcd`).
  - `TestNotConfigured`: `MatchIngredients` bez klucza → `ErrNotConfigured`, bez wywołania klienta.
- [ ] **Step 2: Uruchom**, FAIL.
- [ ] **Step 3: Zaimplementuj** pliki. `http.Client` z `Timeout: 60*time.Second`.
- [ ] **Step 4: Uruchom** `go test ./internal/assistant/... -race`, PASS.
- [ ] **Step 5: Commit** `feat(assistant): OpenRouter client, prompts and validated suggestions`

### Task 5: API i OpenAPI

**Files:**
- Create: `internal/httpapi/assistant.go`, `internal/httpapi/assistant_test.go`
- Modify: `internal/httpapi/server.go` (Deps `Assistant *assistant.Service`, trasy), `internal/httpapi/products.go` (assign, usage, merge), `internal/httpapi/errors.go` (mapowanie błędów asystenta), `cmd/mealplan/main.go` (spięcie), `api/openapi.yaml`, `docs/api.md`

**Interfaces:**
- Trasy (wszystkie `s.private`):
  - `GET /api/v1/household/assistant` → `{configured, keyHint, model, defaultModel}`
  - `PUT /api/v1/household/assistant` ← `{key?: string, model: string}` → jak GET. Brak `key` w JSON oznacza „nie zmieniaj” (użyj `*string`).
  - `DELETE /api/v1/household/assistant` → 204 (usuwa klucz, model zostaje)
  - `POST /api/v1/household/assistant/test` → 204 albo błąd
  - `POST /api/v1/assistant/ingredients` ← `{ingredients:[{name,quantity,unit}]}` → `{suggestions:[{name, kind, productId?, productName?, newProduct?{name,per100g,pieceGrams,densityGPerMl,aliases}, reason}]}`
  - `GET /api/v1/assistant/ingredient-candidates` → `{candidates:[{name, normalizedName, currentProductId, recipeIds, recipeTitles}]}`
  - `GET /api/v1/assistant/product-batches` → `{batches:[[id,…],…]}`
  - `POST /api/v1/assistant/duplicates` ← `{productIds:[…]}` → `{groups:[{keepId, absorbIds, reason}]}`
  - `POST /api/v1/products/{id}/assign` ← `{name, recipeIds}` → `{updated, aliasAdded, aliasConflict}`
  - `GET /api/v1/products/{id}/usage` → `{recipes, pantry, foodLog, shopping}`
  - `POST /api/v1/products/{id}/merge` ← `{absorbIds}` → `{repointed:{…}, skippedAliases}`
- Błędy: `ErrNotConfigured` → 409 `assistant_not_configured`; `ErrBadKey` → 502 `assistant_bad_key`; `ErrNoCredit` → 502 `assistant_no_credit`; `ErrUnavailable` → 502 `assistant_unavailable`; brak uprawnień → 403 (jak dziś w handlerach członków).
- Handlery wołające model (`ingredients`, `duplicates`, `test`) używają `importLimit` i przedłużają `SetWriteDeadline` (Global Constraints).

- [ ] **Step 1: Failing testy** (wzór: `importer_test.go`, `members_test.go`). Fałszywy `assistant.Client` wstrzyknij przez `assistant.NewService`.
  - `TestAssistantSettingsAdminOnly` (Review Focus 4): członek → PUT 403, DELETE 403; GET 200 bez pola `key` w surowym JSON-ie (sprawdź `strings.Contains(body, "sk-or")` == false).
  - `TestAssistantIngredientsMapsErrors`: klient zwraca `ErrBadKey` → 502 `assistant_bad_key`; brak klucza → 409.
  - `TestProductMergeEndpoint` i `TestProductAssignEndpoint` dla szczęśliwej ścieżki oraz 404 dla obcego produktu.
  - `TestRoutesMatchOpenAPI` przechodzi.
- [ ] **Step 2: Uruchom**, FAIL.
- [ ] **Step 3: Zaimplementuj** handlery, wpisy OpenAPI i sekcję w `docs/api.md`. W `main.go`: `assistant.NewService(st, products, assistant.NewOpenRouter(&http.Client{Timeout: 60 * time.Second}))`.
- [ ] **Step 4: Uruchom** `.superpowers/bin/dkgo go test ./... -race` i `.superpowers/bin/dkgo go vet ./...`, PASS.
- [ ] **Step 5: Commit** `feat(api): assistant settings, suggestions, assign and merge endpoints`

### Task 6: Front — ustawienia i store asystenta

**Files:**
- Create: `web/src/stores/assistant.ts`, `web/src/stores/assistant.test.ts`, `web/src/components/AssistantSettings.vue`, `web/src/components/AssistantSettings.test.ts`
- Modify: `web/src/api/types.ts`, `web/src/messages.ts`, `web/src/views/MembersView.vue` (sekcja pod ustawieniem porcji, widoczna dla wszystkich; edycja tylko gdy `session.isAdmin`)

**Interfaces:**
- Produces (`useAssistant`): state `{ configured: boolean, keyHint: string, model: string, defaultModel: string, loaded: boolean }`; akcje `load()`, `save(key: string | undefined, model: string)`, `remove()`, `test(): Promise<void>`, `matchIngredients(qs: IngredientQuery[]): Promise<Suggestion[]>`. Ta ostatnia sama dzieli na partie po 25 i skleja wyniki w kolejności. Dalej: `candidates()`, `productBatches()`, `duplicates(ids)`, `assign(productId, name, recipeIds)`, `usage(id)`, `merge(keepId, absorbIds)`.
- Typy TS odpowiadają JSON-owi z Task 5.
- Teksty (`t.*`):
  - `assistantTitle: 'Asystent produktów (OpenRouter)'`
  - `assistantKey: 'Klucz API OpenRouter'`
  - `assistantModel: 'Model'`
  - `assistantKeySet: (hint) => \`Klucz ustawiony, kończy się na …${hint}\``
  - `assistantPrivacy: 'Nazwy składników i produktów są wysyłane do OpenRoutera, żeby dopasować je do Twojej bazy.'`
  - `assistantTest: 'Sprawdź połączenie'`, `assistantOk: 'Połączenie działa'`
  - błędy: `assistant_bad_key: 'OpenRouter nie przyjął klucza'`, `assistant_no_credit: 'Brak środków na koncie OpenRouter'`, `assistant_unavailable: 'OpenRouter nie odpowiada. Spróbuj później.'`, `assistant_not_configured: 'Asystent nie ma klucza API'`

- [ ] **Step 1: Failing testy**
  - `assistant.test.ts`: `matchIngredients` z 30 zapytaniami robi 2 wywołania (25 + 5) i zwraca 30 wyników w kolejności.
  - `AssistantSettings.test.ts`: admin widzi pole klucza i „Zapisz”; członek widzi tylko „Klucz ustawiony, kończy się na …abcd” bez pola; zapis bez wpisanego klucza wysyła body bez `key`.
- [ ] **Step 2: Uruchom**, FAIL.
- [ ] **Step 3: Zaimplementuj**. Pole klucza `type="password"`, `autocomplete="off"`, nigdy nie wypełniane z serwera.
- [ ] **Step 4: Uruchom** `npx vitest run` i `npm run build`, PASS.
- [ ] **Step 5: Commit** `feat(web): assistant settings for the household`

### Task 7: Front — podpowiedzi w edytorze przepisu

**Files:**
- Create: `web/src/components/ProductSuggestion.vue`
- Modify: `web/src/views/RecipeEditView.vue`, `web/src/messages.ts`
- Test: `web/src/views/RecipeEditView.test.ts`

**Interfaces:**
- Consumes: `useAssistant().matchIngredients`, `assign`, `ProductPicker` (`suggest-name`, `allow-create` już są), `ProductDialog` (`initial-name`). Dodaj opcjonalny prop `initial?: Partial<Product>` wypełniający wartości i aliasy oraz komunikat „Wartości to szacunek asystenta — sprawdź przed zapisem”.
- Zachowanie:
  - Gdy edytor jest otwarty ze szkicu importu (`fromImport`) i `assistant.configured`, po załadowaniu wysyła składniki bez `productId` (niepusta nazwa) do `matchIngredients`.
  - Propozycja `existing` z `reason === 'exact'` ustawia `productId` od razu, bo to dzisiejsze dokładne dopasowanie, a nie decyzja modelu. Pozostałe pokazuje `ProductSuggestion` pod pickerem: „Propozycja: {nazwa}” albo „Propozycja: {nazwa} (nowy produkt)”, z przyciskami „Zatwierdź” i „Odrzuć”.
  - „Zatwierdź” przy `existing`: ustawia `ing.productId` i woła `assign(productId, ing.name, [])`. Przy `aliasConflict` pokazuje `notice` „„{nazwa}” jest już aliasem produktu {X}”.
  - „Zatwierdź” przy `new` otwiera `ProductDialog` z `initial`. Po zapisaniu wykonuje to samo co `existing` z nowym id.
  - Błąd modelu daje jeden `v-alert` typu info nad składnikami: „Podpowiedzi produktów są teraz niedostępne: {błąd}”. Formularz działa dalej.

- [ ] **Step 1: Failing testy** w `RecipeEditView.test.ts` (wzór: istniejący `show()` z `fakeApi`):
  - `'suggests a product for an imported ingredient and learns the alias on approve'`: szkic z „czerwona cebula” i API `ingredients` zwraca `existing p9`. Po kliknięciu „Zatwierdź” jest wywołanie `POST /products/p9/assign` z `{name:'czerwona cebula', recipeIds:[]}`, a picker ma `modelValue === 'p9'`.
  - `'keeps the form usable when suggestions fail'` (Review Focus 5): API rzuca błąd z kodem `assistant_unavailable` → widoczny tekst niedostępności, „Zapisz” działa.
  - `'does not call the assistant when it is not configured'`.
- [ ] **Step 2: Uruchom**, FAIL.
- [ ] **Step 3: Zaimplementuj**.
- [ ] **Step 4: Uruchom** `npx vitest run` i `npm run build`, PASS.
- [ ] **Step 5: Commit** `feat(web): product suggestions for imported recipes`

### Task 8: Front — „Sprawdź przepisy”

**Files:**
- Create: `web/src/views/RecipeCheckView.vue`, `web/src/views/RecipeCheckView.test.ts`
- Modify: `web/src/router.ts` (`recipes/check` → `recipe-check`, **przed** `recipes/:id`), `web/src/views/RecipesView.vue` (przycisk „Sprawdź przepisy” w nagłówku, tylko gdy `assistant.configured`), `web/src/messages.ts`

**Zachowanie:**
- Wejście pobiera `candidates()`. Pusto → `EmptyState` „Wszystkie składniki mają produkty”.
- Kandydaci idą do `matchIngredients` partiami. Postęp „Sprawdzam… {i} z {n}” (`v-progress-linear` z `aria-label`) i przycisk „Przerwij” (przestaje wysyłać kolejne partie).
- Wiersz: nazwa składnika, przepisy (tytuły, maks 3 + „i {k} innych”), propozycja, a przy podejrzanym przypisaniu także aktualny produkt („teraz: cebula”). Akcje: „Zatwierdź”, „Odrzuć” i „Inny produkt” (`ProductPicker` z `allow-create`).
- Zatwierdzenie woła `assign(productId, name, recipeIds)`, a wiersz znika z notice „Przypisano {nazwa} w {n} przepisach”. Propozycje `none` nie są pokazywane, tylko licznik „{k} składników bez propozycji”.
- „Zatwierdź wszystkie” obejmuje wyłącznie propozycje `existing` i jest wyłączone w trakcie sprawdzania.

- [ ] **Step 1: Failing testy:** dwa kandydaci (jeden `existing`, jeden `new`); „Zatwierdź wszystkie” robi dokładnie jedno `assign`, a wiersz `new` zostaje. „Przerwij” po pierwszej partii → brak drugiego wywołania `ingredients`.
- [ ] **Step 2–4:** uruchom, zaimplementuj, `vitest` i `build` PASS.
- [ ] **Step 5: Commit** `feat(web): check recipes for unmatched ingredients`

### Task 9: Front — „Sprawdź produkty” i „Scal z…”

**Files:**
- Create: `web/src/views/ProductCheckView.vue`, `web/src/views/ProductCheckView.test.ts`, `web/src/components/MergeDialog.vue`
- Modify: `web/src/router.ts` (`products/check` → `product-check`), `web/src/views/ProductsView.vue` (przycisk „Sprawdź produkty” tylko gdy `configured`; „Scal z…” w menu pozycji zawsze), `web/src/messages.ts`, `web/src/stores/products.ts` (po scaleniu przeładuj katalog)

**Zachowanie:**
- `productBatches()`, potem `duplicates(batch)` po kolei, z postępem i „Przerwij” jak w Task 8.
- Grupa pokazuje produkty jako `v-radio-group`: zaznaczony = zostaje (domyślnie `keepId`), reszta = do scalenia. Obok każdego `usage` w formie „3 przepisy · 1 w spiżarni · 2 w dzienniku” (użyj `form()` z `polish.ts`). Akcje: „Scal” i „Odrzuć”.
- „Scal” → `merge(keep, absorb)` → notice „Scalono w „{nazwa}””, a gdy `skippedAliases` niepuste, dopisek „pominięte aliasy: …”.
- `MergeDialog` (ręczne „Scal z…”) to autocomplete produktu do wchłonięcia, podgląd usage i potwierdzenie „Scal”. Tekst ostrzegawczy: „„{B}” zniknie, a jego przepisy, spiżarnia i dziennik przejdą do „{A}”.”

- [ ] **Step 1: Failing testy:** zmiana radio na drugi produkt wysyła `merge` z zamienionymi `keep`/`absorb`. `MergeDialog` wysyła `POST /products/A/merge` z `{absorbIds:['B']}`.
- [ ] **Step 2–4:** uruchom, zaimplementuj, `vitest` i `build` PASS.
- [ ] **Step 5: Commit** `feat(web): find and merge duplicate products`

### Task 10: Dokumentacja i weryfikacja end-to-end

**Files:**
- Modify: `docs/import.md`, `docs/nutrition.md`, `docs/security.md`, `CHANGELOG.md` (sekcja „Asystent produktów”), `docs/docs_test.go`, jeśli pilnuje listy sekcji

- [ ] **Step 1:** opisz w dokumentacji: dopasowanie przy imporcie, aliasy z zatwierdzeń, scalanie, przechowywanie klucza (jawnie w bazie, nigdy nie zwracany; kopie zapasowe zawierają klucz) i jakie dane trafiają do OpenRoutera.
- [ ] **Step 2:** uruchom `go test ./... -race`, `vitest`, `npm run build` i `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` w `dkgo`. Wszystko PASS.
- [ ] **Step 3: Weryfikacja na localhost.** Uruchom `docker build -t mealplan:assist .` i kontener na `127.0.0.1:18104` z `MEALPLAN_COOKIE_SECURE=false`, potem seed skryptami z `$SP`. Prawdziwego klucza nie wpisuj, OpenRouter nie jest dostępny w testach. Headless Playwright sprawdza:
  - ustawienia: zapis fałszywego klucza `sk-or-test-0000`, widoczne „…0000”, „Sprawdź połączenie” pokazuje „OpenRouter nie przyjął klucza”;
  - import tekstu ze składnikiem „czerwona cebula” → edytor pokazuje informację o niedostępności i da się zapisać;
  - „Scal z…” na dwóch produktach testowych, przepis przechodzi na produkt, który zostaje;
  - brak `pageerror`, zrzuty telefon i desktop.
  
  Kontener usuń po teście. Nie dotykaj kontenera `mealplan` ani wolumenu `mealplan-data`.
- [ ] **Step 4: Commit** `docs: product assistant`
