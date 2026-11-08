# Wartości odżywcze (Część 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Każdy domownik z włączonym śledzeniem widzi pod każdym dniem sumę kalorii, białka, węglowodanów i tłuszczu z jednej porcji każdego zaplanowanego posiłku i własnych dopisanych produktów, w porównaniu z własnym celem.

**Architecture:** Nowa baza produktów gospodarstwa (makro na 100 g, gramatura, gęstość, nazwy alternatywne) z danymi startowymi generowanymi z USDA FoodData Central (domena publiczna). Nowy pakiet `internal/nutrition` liczy wartości przepisu na porcję (per wartość: z przepisu / ręczne, inaczej wyliczone ze składników) i podsumowanie dnia; wartości wyliczone nie są zapisywane, tylko liczone przy odczycie. Importer czyta `nutrition` z JSON-LD i linię „Wartości na porcję:” z tekstu.

**Tech Stack:** Go 1.26 (`database/sql`, SQLite + Postgres, migracje w `internal/store/migrations`), Vue 3 + Vuetify 3 + Pinia + Vitest, OpenAPI w `api/openapi.yaml`.

**Spec:** `docs/superpowers/specs/2026-11-08-nutrition-mealprep-inventory-design.md` (§1–4, §7). Części 2 i 3 mają osobne plany.

## Global Constraints

- Gałąź: `feat/nutrition` z `docs/nutrition-mealprep-inventory-spec` (ta = `master` 6c29dfd + commit specu). Commity z tożsamością noreply z konfiguracji repo; każdy commit kończy się linią `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Bez pushowania bez prośby.
- Host nie ma Go ani Node: `.superpowers/bin/dkgo go …` i `.superpowers/bin/dknode npm …` (w `web/`); po buildzie `touch web/dist/.gitkeep`.
- Migracje przenośne na SQLite i Postgres; testy store muszą przejść na obu (CI ma job `go (postgres)` z `MEALPLAN_TEST_DSN`; lokalnie tylko SQLite — odnotuj to w raporcie).
- `TestRoutesMatchOpenAPI` zielony po każdej zmianie tras. Błędy w formacie `{"code","message"}`; walidacja przez `domain.ErrInvalid` → `422 invalid_input`.
- Gospodarstwo jest tenantem: produkty, przepisy i wpisy są izolowane per `household_id`; ustawienia odżywcze i dziennik należą do pary (użytkownik, gospodarstwo) i nie są widoczne dla innych domowników.
- Makro produktu zawsze **na 100 g**; gramatura sztuki w gramach; wartości przepisu zawsze **na jedną porcję**.
- Podsumowanie nigdy nie zaniża po cichu: każdy brak (posiłek bez wartości, składnik niepoliczony) jest widoczny.
- Dane startowe tylko z USDA FoodData Central (domena publiczna, CC0); żadnych wartości wpisanych „z pamięci” i żadnych danych z tabel chronionych prawem autorskim.
- Tekst dla użytkownika po polsku; kod, komentarze i identyfikatory po angielsku; komentarze oszczędne.
- Wynik importu to dane: w UI tylko `{{ }}`, nigdy `v-html`.

## Review Focus

1. Składnik w jednostce sztukowej (`3 jajka`, `2 ząbki czosnku`) dla produktu bez gramatury: nie liczy się jako 0, tylko trafia do `unmatched` i oznacza wynik jako przybliżony (Task 1).
2. Posiłek z wolnym tekstem w dniu: suma pokazuje go jako brakujący (`missingMeals`), a nie jako 0 kcal (Task 7).
3. Przepis z częściowymi wartościami ze źródła (kcal i białko, bez węglowodanów i tłuszczu): podane wartości wygrywają, brakujące są wyliczone, a te, których nie da się wyliczyć, mają źródło `missing` (Task 1, Task 5).
4. Usunięcie produktu: link składnika jest czyszczony (wraca dopasowanie automatyczne), a produkt użyty w dzienniku nie może zostać usunięty (`409 in_use`) (Task 3, Task 4).
5. Drugi domownik nie widzi celu ani dziennika pierwszego i nie może usunąć jego wpisu (`404`) (Task 7).

---

## File Structure

Backend:
- `internal/domain/domain.go` — nowe typy `Macros`, `Product`, `NutritionSettings`, `FoodLogEntry`; `Recipe.Nutrition`, `Ingredient.ProductID`.
- `internal/nutrition/grams.go` — przeliczanie ilości na gramy.
- `internal/nutrition/match.go` — dopasowanie składnika do produktu.
- `internal/nutrition/recipe.go` — wartości przepisu na porcję.
- `internal/nutrition/day.go` + `service.go` — podsumowanie dnia, ustawienia, dziennik.
- `internal/nutrition/seed/` — `mapping.csv` (ręczna lista), `products.csv` (wygenerowane), `gen/main.go` (generator), `seed.go` (`//go:embed products.csv`).
- `internal/product/service.go` — CRUD produktów i zasiew startowy.
- `internal/store/migrations/0005_nutrition.sql`, `internal/store/products.go`, `internal/store/nutrition.go`; zmiany w `internal/store/recipes.go`.
- `internal/httpapi/products.go`, `internal/httpapi/nutrition.go`; zmiany w `recipes.go`, `importer.go`, `server.go`, `api/openapi.yaml`, `cmd/mealplan/main.go`.
- `internal/importer/nutrition.go` — odczyt wartości z JSON-LD i z tekstu.

Frontend:
- `web/src/api/types.ts`, `web/src/stores/products.ts`, `web/src/stores/nutrition.ts`.
- `web/src/views/AccountView.vue`, `web/src/views/ProductsView.vue`.
- `web/src/components/DaySummary.vue`, `MealNutritionDialog.vue`, `ProductPicker.vue`, `FoodLogDialog.vue`.
- Zmiany: `TodayView.vue`, `PlanView.vue`, `RecipeEditView.vue`, `importDraft.ts`, `stores/recipes.ts`, `router.ts`, `layouts/AppShell.vue`, `messages.ts`.

Docs: `docs/nutrition.md` (nowy), `docs/import.md`, `docs/api.md`, `CHANGELOG.md`, `web/public/claude-skill/SKILL.md`.

---

### Task 1: Pakiet `nutrition` — gramy, dopasowanie, wartości przepisu

**Files:**
- Modify: `internal/domain/domain.go`
- Create: `internal/nutrition/grams.go`, `match.go`, `recipe.go`
- Test: `internal/nutrition/grams_test.go`, `match_test.go`, `recipe_test.go`

**Interfaces:**
- Produces (domain):
  - `type Macros struct { Kcal, Protein, Carbs, Fat *float64 }` (nil = nieznane)
  - `type Product struct { ID, HouseholdID, Name, NormalizedName string; Per100g Macros; PieceGrams *float64; DensityGPerML *float64; Aliases []string; CreatedAt, UpdatedAt time.Time }`
  - `Recipe.Nutrition Macros` (wartości podane / ręczne na porcję), `Ingredient.ProductID string` (zapamiętany wybór, `""` = brak)
- Produces (nutrition):
  - `func Grams(quantity float64, unit string, p domain.Product) (float64, bool)`
  - `type Matcher struct{…}`; `func NewMatcher(products []domain.Product) *Matcher`; `func (m *Matcher) Match(ing domain.Ingredient) (domain.Product, bool)`
  - `type Source string` z `SourceRecipe = "recipe"`, `SourceComputed = "computed"`, `SourceMissing = "missing"`
  - `type Sources struct { Kcal, Protein, Carbs, Fat Source }`
  - `type RecipeNutrition struct { PerPortion domain.Macros; Sources Sources; Approximate bool; Unmatched []string }`
  - `func ForRecipe(r domain.Recipe, m *Matcher) RecipeNutrition`

- [ ] **Step 1: Write the failing tests**

`grams_test.go` — tabela `TestGrams`: produkt bez gramatury i gęstości oraz produkty `jajko{PieceGrams:50}`, `olej{DensityGPerML:0.92}`, `mleko{DensityGPerML:1.03}`:
- `(250,"g")`→250; `(0.5,"kg")`→500; `(1,"l",mleko)`→1030; `(200,"ml",bez gęstości)`→200
- `(2,"łyżka",olej)`→27.6 (2×15 ml×0.92); `(1,"łyżeczka")`→5; `(1,"szklanka")`→250
- `(3,"szt",jajko)`→150; `(2,"ząbek",bez gramatury)`→`ok=false`; `(1,"opakowanie",{PieceGrams:400})`→400
- `(1,"szczypta")`→0.5; `(1,"garść")`→30; `(1,"cup")`→`ok=false`
Jednostki sztukowe korzystające z gramatury: `szt, ząbek, plaster, kostka, opakowanie, puszka, kromka, główka, pęczek`.

`match_test.go` — `TestMatchPrefersRememberedProduct` (ingredient z `ProductID` produktu B dopasowuje B, nawet gdy nazwa pasuje do A); `TestMatchByNormalizedNameAndAlias` (`"Mąka pszenna"` → produkt o nazwie `mąka pszenna`; `"mąki"` → produkt z aliasem `mąki`; dopasowanie bez rozróżniania wielkości liter i z ujednoliconymi spacjami przez `units.NormalizeName`); `TestMatchUnknownProductID` (ProductID nieistniejącego produktu → dopasowanie po nazwie); `TestNoMatch`.

`recipe_test.go`:
- `TestForRecipeComputesPerPortion`: przepis 4 porcje, `400 g makaron` (makaron 350 kcal/12 B/72 W/1.5 T na 100 g) i `2 łyżki oliwa` (884 kcal, 0/0/100, gęstość 0.92) → kcal na porcję = (400×3.5 + 27.6×8.84)/4 = 410.996…, `Sources` wszystkie `computed`, `Approximate=false`.
- `TestGivenValuesWinPerValue`: `Recipe.Nutrition{Kcal:520, Protein:32}` → kcal 520 i białko 32 ze źródłem `recipe`, węglowodany i tłuszcz `computed`.
- `TestUnmatchedMakesApproximate`: składnik `3 jajka` bez produktu z gramaturą → `Unmatched=["jajka"]`, `Approximate=true`; wartości z pozostałych składników są nadal policzone.
- `TestNoQuantityIsZeroWithoutFlag`: `sól` bez ilości → nie trafia do `Unmatched`, `Approximate=false`.
- `TestNothingComputable`: przepis bez dopasowań i bez wartości podanych → wszystkie `Sources` `missing`, `PerPortion` z nil.
- `TestProductWithoutMacrosIsUnmatched`: produkt bez makro (`Per100g` nil) → składnik w `Unmatched`.

- [ ] **Step 2: Run tests, verify they fail**

Run: `.superpowers/bin/dkgo go test ./internal/nutrition/`
Expected: FAIL (build: pakiet nie istnieje).

- [ ] **Step 3: Implement `domain` types, `Grams`, `Matcher`, `ForRecipe`**

Stałe przeliczników w `grams.go`: łyżka 15 ml, łyżeczka 5 ml, szklanka 250 ml, gęstość domyślna 1 g/ml, szczypta 0.5 g, garść 30 g. `ForRecipe`: suma gramów × makro/100 dla każdego składnika z ilością i dopasowanym produktem z makro, podzielona przez `BaseServings`; wartość wyliczona jest `nil`, gdy żaden składnik jej nie dostarczył. Wartości zaokrąglone przy wyświetlaniu, nie w obliczeniach.

- [ ] **Step 4: Run tests, verify they pass**

Run: `.superpowers/bin/dkgo go test ./internal/nutrition/ && .superpowers/bin/dkgo go vet ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

`git commit -m "feat(nutrition): grams, product matching and per-portion values"`

---

### Task 2: Dane startowe produktów z USDA FoodData Central

**Files:**
- Create: `internal/nutrition/seed/mapping.csv`, `internal/nutrition/seed/gen/main.go`, `internal/nutrition/seed/products.csv`, `internal/nutrition/seed/seed.go`
- Test: `internal/nutrition/seed/seed_test.go`

**Interfaces:**
- Produces: `func seed.Products() ([]domain.Product, error)` (z osadzonego `products.csv`, bez ID i gospodarstwa)

- [ ] **Step 1: Write the failing test**

`seed_test.go`: `TestSeedProductsAreConsistent` — co najmniej 150 produktów; nazwy unikalne po `units.NormalizeName`; aliasy unikalne w całym zestawie i różne od nazw innych produktów; każde makro ≥ 0, białko+węgle+tłuszcz ≤ 100; **zgodność energii**: |kcal − (4×białko + 4×węgle + 9×tłuszcz)| ≤ max(25, 15% kcal) dla każdego produktu (wychwytuje przestawione kolumny i wymyślone liczby); `PieceGrams` i `DensityGPerML`, jeśli podane, w zakresie (0, 5000] i (0.2, 2]. Zawiera co najmniej: `mąka pszenna`, `jajko` (z gramaturą), `ryż biały`, `makaron`, `pierś z kurczaka`, `mleko`, `masło`, `oliwa` (z gęstością), `cukier`, `ziemniaki`, `cebula`, `czosnek` (z gramaturą ząbka), `pomidor`, `marchew`, `ser żółty`, `jogurt naturalny`, `płatki owsiane`, `banan`, `jabłko`, `fasola czerwona`.

- [ ] **Step 2: Run, verify it fails** (`go test ./internal/nutrition/seed/` → build fail).

- [ ] **Step 3: Write the generator and the mapping**

`gen/main.go` (wywołanie: `go run ./internal/nutrition/seed/gen -fdc <katalog z rozpakowanym zestawem FDC SR Legacy CSV> -mapping internal/nutrition/seed/mapping.csv -out internal/nutrition/seed/products.csv`) czyta `food.csv` i `food_nutrient.csv` i bierze wartości na 100 g dla nutrientów: energia kcal `1008`, białko `1003`, tłuszcz `1004`, węglowodany `1005`. `mapping.csv` (kolumny `name;aliases;fdc_id;piece_grams;density`, aliasy rozdzielone `|`, np. odmiany „mąki”, „mąką”) jest pisany ręcznie: polska nazwa → `fdc_id` znaleziony po opisie w `food.csv`. Gramatury (jajko, ząbek czosnku, cebula…) i gęstości (oleje, mleko, miód) to średnie podane w `mapping.csv`, w nagłówku pliku komentarz ze źródłem każdej (np. rubryka „portion” z `food_portion.csv` FDC). Zestaw FDC pobierz do katalogu poza repo (`/tmp/claude-1000/-home-silly/0e36aff8-a9b5-44e4-930f-4e051e485ce4/scratchpad/fdc/`), nie commituj go. `products.csv` jest wynikiem generatora i jest commitowany; jego nagłówek mówi, że plik jest generowany, źródło i licencja (USDA FoodData Central, CC0 1.0).

- [ ] **Step 4: Run the generator and the test**

Run: generator jak wyżej, potem `.superpowers/bin/dkgo go test ./internal/nutrition/seed/`
Expected: PASS. Jeśli test energii odrzuca produkt, popraw `fdc_id` w mapowaniu, nie liczby.

- [ ] **Step 5: Commit** (`feat(nutrition): seed product data from USDA FoodData Central`)

---

### Task 3: Migracja i store

**Files:**
- Create: `internal/store/migrations/0005_nutrition.sql`, `internal/store/products.go`, `internal/store/nutrition.go`
- Modify: `internal/store/recipes.go`, `internal/store/db_test.go` (liczba migracji 4 → 5)
- Test: `internal/store/products_test.go`, `internal/store/nutrition_test.go`, `internal/store/recipes_test.go`

**Interfaces:**
- Consumes: typy z Task 1.
- Produces (domain): `type NutritionSettings struct { UserID, HouseholdID string; Enabled bool; Goal Macros }`; `type FoodLogEntry struct { ID, HouseholdID, UserID, Date, ProductID string; Quantity float64; Unit string; CreatedAt time.Time }` (Unit `"g"` albo `"szt"`).
- Produces (store, metody `*Store`):
  - `CreateProduct(ctx, domain.Product) error`, `UpdateProduct(ctx, domain.Product) error`, `DeleteProduct(ctx, householdID, id string) error` (`ErrInUse` gdy produkt jest w `food_log`), `ProductByID(ctx, householdID, id string) (domain.Product, error)`, `ListProducts(ctx, householdID string) ([]domain.Product, error)` (z aliasami), `SeedProducts(ctx, householdID string, ps []domain.Product) (bool, error)` (wstawia tylko gdy gospodarstwo nie było jeszcze zasiane; zwraca czy zasiało)
  - `NutritionSettings(ctx, householdID, userID string) (domain.NutritionSettings, error)` (brak wiersza = wyłączone, puste cele), `SaveNutritionSettings(ctx, domain.NutritionSettings) error`
  - `AddFoodLog(ctx, domain.FoodLogEntry) error`, `DeleteFoodLog(ctx, householdID, userID, id string) error` (`ErrNotFound` dla cudzego wpisu), `FoodLog(ctx, householdID, userID, from, to string) ([]domain.FoodLogEntry, error)`
  - `RecipesByIDs` i `RecipeByID` zwracają `Nutrition` i `Ingredient.ProductID`.

- [ ] **Step 1: Write the failing tests**

- `TestProductCRUDAndAliases`: create z aliasami, list, update zmienia aliasy, unikalność nazwy znormalizowanej w gospodarstwie (`ErrConflict`), te same nazwy w dwóch gospodarstwach są dozwolone.
- `TestDeleteProductClearsIngredientLinkAndRespectsLog`: składnik z `ProductID` → po usunięciu produktu `ProductID == ""`; produkt z wpisem w dzienniku → `ErrInUse`.
- `TestSeedProductsOnlyOnce`: pierwsze wywołanie wstawia i zwraca `true`; drugie (nawet po usunięciu wszystkich produktów) zwraca `false` i nic nie wstawia.
- `TestRecipeNutritionRoundTrip`: zapis i odczyt `Nutrition` z częściowymi wartościami (nil zostaje nil) oraz `ProductID` składnika; update nadpisuje.
- `TestNutritionSettingsAndFoodLogIsolation`: ustawienia dwóch użytkowników niezależne; `FoodLog` zwraca tylko wpisy danego użytkownika i zakresu dat; `DeleteFoodLog` cudzego wpisu → `ErrNotFound`.

- [ ] **Step 2: Run, verify fail** (`.superpowers/bin/dkgo go test ./internal/store/`).

- [ ] **Step 3: Write `0005_nutrition.sql` and the store code**

Migracja (przenośna, jedna instrukcja na `;`): tabela `product` (`id`, `household_id` → `household` ON DELETE CASCADE, `name`, `normalized_name`, `kcal`, `protein`, `carbs`, `fat` DOUBLE PRECISION NULL na 100 g, `piece_grams`, `density` DOUBLE PRECISION NULL, `created_at`, `updated_at` BIGINT, UNIQUE(`household_id`,`normalized_name`)); `product_alias` (`household_id`, `product_id` → `product` ON DELETE CASCADE, `alias`, PRIMARY KEY(`household_id`,`alias`)); `ALTER TABLE household ADD COLUMN products_seeded INTEGER NOT NULL DEFAULT 0`; `ALTER TABLE recipe ADD COLUMN` `kcal`, `protein`, `carbs`, `fat` DOUBLE PRECISION; `ALTER TABLE ingredient ADD COLUMN product_id TEXT REFERENCES product(id) ON DELETE SET NULL`; `nutrition_settings` (`user_id` → `app_user` CASCADE, `household_id` → `household` CASCADE, `enabled` INTEGER NOT NULL DEFAULT 0, `goal_kcal`, `goal_protein`, `goal_carbs`, `goal_fat` DOUBLE PRECISION NULL, PRIMARY KEY(`user_id`,`household_id`)); `food_log` (`id`, `household_id` CASCADE, `user_id` → `app_user` CASCADE, `date` TEXT, `product_id` → `product` ON DELETE RESTRICT, `quantity` DOUBLE PRECISION NOT NULL CHECK > 0, `unit` TEXT NOT NULL CHECK IN ('g','szt'), `created_at` BIGINT) z indeksem (`household_id`,`user_id`,`date`). SQLite: sprawdź, czy `ALTER TABLE … ADD COLUMN … REFERENCES` jest akceptowane przy włączonych kluczach obcych (jest, gdy domyślna wartość to NULL); jeśli nie, przebuduj tabelę `ingredient` w migracji.

- [ ] **Step 4: Run, verify pass** — `.superpowers/bin/dkgo go test -count=1 ./internal/store/` (SQLite; Postgres w CI).

- [ ] **Step 5: Commit** (`feat(store): products, recipe nutrition, settings and food log`)

---

### Task 4: Produkty — serwis i API

**Files:**
- Create: `internal/product/service.go`, `internal/httpapi/products.go`
- Modify: `internal/httpapi/server.go` (`Deps.Products`, trasy), `api/openapi.yaml`, `cmd/mealplan/main.go`, `internal/httpapi/server_test.go` (`newEnv`)
- Test: `internal/product/service_test.go`, `internal/httpapi/products_test.go`

**Interfaces:**
- Consumes: store z Task 3, `seed.Products()` z Task 2.
- Produces: `product.Service` z `List(ctx, householdID, q string) ([]domain.Product, error)` (zasiewa przy pierwszym wywołaniu dla gospodarstwa; `q` filtruje po nazwie i aliasach, bez wielkości liter), `All(ctx, householdID string) ([]domain.Product, error)` (też zasiewa), `Create`, `Update(ctx, householdID, id string, in Input)`, `Delete`; `type Input struct { Name string; Per100g domain.Macros; PieceGrams, DensityGPerML *float64; Aliases []string }`.
- Trasy: `GET /api/v1/products?q=`, `POST /api/v1/products`, `PUT /api/v1/products/{id}`, `DELETE /api/v1/products/{id}`. JSON produktu: `{id, name, per100g:{kcal,protein,carbs,fat}, pieceGrams, densityGPerMl, aliases}` (wartości nullable).

- [ ] **Step 1: Write the failing tests**

Serwis: walidacja (nazwa 1–200 znaków, makro ≥ 0 i ≤ 900 kcal / ≤ 100 g na 100 g, suma B+W+T ≤ 100, gramatura (0,5000], gęstość (0.2,2], ≤ 20 aliasów po ≤ 200 znaków, aliasy znormalizowane i bez duplikatów, alias nie może być nazwą innego produktu → `ErrConflict`); `List` zasiewa raz i filtruje po `q`. HTTP: CRUD, `409 in_use` przy usuwaniu produktu z dziennika, izolacja gospodarstw (`404`), `TestRoutesMatchOpenAPI`.

- [ ] **Step 2–4:** fail → implementacja → pass (`.superpowers/bin/dkgo go test ./internal/product/ ./internal/httpapi/`).

- [ ] **Step 5: Commit** (`feat(api): household product database`)

---

### Task 5: Wartości przepisu w API

**Files:**
- Modify: `internal/recipe/service.go` (Input i walidacja), `internal/httpapi/recipes.go`, `api/openapi.yaml`, `cmd/mealplan/main.go`
- Test: `internal/recipe/service_test.go`, `internal/httpapi/recipes_test.go`

**Interfaces:**
- Consumes: `nutrition.ForRecipe`, `nutrition.NewMatcher`, `product.Service.All`.
- Produces: `recipe.Input.Nutrition domain.Macros`, `recipe.IngredientInput.ProductID string`. JSON przepisu: wejście/wyjście `nutrition:{kcal,protein,carbs,fat}` (podane/ręczne, nullable) i `ingredients[].productId` (nullable); tylko wyjście `GET /recipes/{id}`: `nutritionPerPortion:{values:{kcal,protein,carbs,fat}, sources:{kcal,protein,carbs,fat}, approximate, unmatched:[]}`. Lista `GET /recipes` nie liczy wartości.

- [ ] **Step 1: Write the failing tests**

Walidacja: każda wartość nil albo 0–5000 (kcal) / 0–500 (g); `productId` musi należeć do gospodarstwa (`404` przy cudzym), pusty = brak. HTTP: zapis częściowych wartości i odczyt `nutritionPerPortion` z mieszanymi źródłami (Review Focus 3); `productId` zapamiętany i użyty w obliczeniu; brak `nutrition` w żądaniu = wszystkie nil (zgodność ze starymi klientami).

- [ ] **Step 2–4:** fail → implementacja → pass.

- [ ] **Step 5: Commit** (`feat(recipe): per-portion nutrition with given and computed values`)

---

### Task 6: Import — wartości z JSON-LD i z tekstu

**Files:**
- Create: `internal/importer/nutrition.go`
- Modify: `internal/importer/draft.go` (`Draft.Nutrition domain.Macros`, walidacja w `finish`), `html.go` (`draftFromLD`), `text.go` (`ParseText`), `internal/httpapi/importer.go` (`draftJSON.nutrition`), `api/openapi.yaml` (`RecipeDraft.nutrition`)
- Test: `internal/importer/nutrition_test.go`, `html_test.go`, `text_test.go`

**Interfaces:**
- Produces: `func parseNutritionLine(line string) (domain.Macros, bool)`, `func nutritionFromLD(v any) domain.Macros`; `Draft.Nutrition`.

- [ ] **Step 1: Write the failing tests**

Linia tekstu (dowolna kolejność i podzbiór, przecinek lub średnik, przecinek dziesiętny): `Wartości na porcję: 520 kcal, białko 32 g, węglowodany 60 g, tłuszcz 14 g` → 520/32/60/14; `Wartości na porcję: 410 kcal; tłuszcz 9,5 g` → kcal 410, tłuszcz 9.5, reszta nil; `Per serving: 300 kcal, protein 20 g, carbs 30 g, fat 10 g`; linia jest zjadana (nie trafia do składników ani kroków); `Wartości na porcję:` bez liczb → ignorowana. JSON-LD: `"nutrition":{"@type":"NutritionInformation","calories":"250 kcal","proteinContent":"10 g","carbohydrateContent":"30,5 g","fatContent":"8 g"}`, `"calories":"250 calories"`, `"calories":250`, brak `nutrition` → nil. `finish` odrzuca (ustawia nil z ostrzeżeniem) wartości poza zakresem z Task 5.

- [ ] **Step 2–4:** fail → implementacja → pass (`go test ./internal/importer/ ./internal/httpapi/`).

- [ ] **Step 5: Commit** (`feat(importer): read nutrition from JSON-LD and the text line`)

---

### Task 7: Ustawienia, dziennik i podsumowanie dnia

**Files:**
- Create: `internal/nutrition/day.go`, `internal/nutrition/service.go`, `internal/httpapi/nutrition.go`
- Modify: `internal/httpapi/server.go`, `api/openapi.yaml`, `cmd/mealplan/main.go`
- Test: `internal/nutrition/day_test.go`, `internal/nutrition/service_test.go`, `internal/httpapi/nutrition_test.go`

**Interfaces:**
- Consumes: store (Task 3), `product.Service.All`, `ForRecipe`, plan entries (`Store.PlanEntries`), `Store.RecipesByIDs`.
- Produces:
  - `nutrition.Service` z `Settings(ctx, p auth.Principal) (domain.NutritionSettings, error)`, `SaveSettings(ctx, p, enabled bool, goal domain.Macros) error`, `AddLog(ctx, p, date, productID string, quantity float64, unit string) (domain.FoodLogEntry, error)`, `DeleteLog(ctx, p, id string) error`, `Days(ctx, p, from, to string) ([]DaySummary, error)` (zakres ≤ 31 dni, daty `YYYY-MM-DD`)
  - `type DaySummary struct { Date string; Total domain.Macros; Goal domain.Macros; Remaining domain.Macros; Meals []MealNutrition; Log []LogNutrition; MissingMeals int; Approximate bool }`
  - `type MealNutrition struct { EntryID, Title string; Slot domain.Slot; Values domain.Macros; Sources Sources; Approximate bool; Missing bool }`
  - `type LogNutrition struct { ID, ProductID, Name string; Quantity float64; Unit string; Values domain.Macros; Missing bool }`
- Trasy: `GET /api/v1/me/nutrition` → `{enabled, goal}`; `PUT /api/v1/me/nutrition` `{enabled, goal}`; `GET /api/v1/me/nutrition/days?from=&to=` → `{days:[…]}` (gdy śledzenie wyłączone: `{enabled:false, days:[]}`); `POST /api/v1/me/food-log` `{date, productId, quantity, unit}` → 201; `DELETE /api/v1/me/food-log/{id}` → 204.

- [ ] **Step 1: Write the failing tests**

`day_test.go`: dzień z dwoma przepisami (każdy liczony jako jedna porcja niezależnie od `Servings` wpisu i od „Zrobione”), jednym wpisem z wolnym tekstem (`MissingMeals=1`, `Missing=true`, nie dodaje 0) i dziennikiem `150 g jogurt` oraz `1 szt baton{PieceGrams:45}`; `Total` = suma; `Remaining` = cel − suma (ujemne dozwolone, nil gdy cel nil); `Approximate` gdy któryś przepis przybliżony; produkt bez makro w dzienniku → `Missing=true`; `1 szt` produktu bez gramatury → `Missing=true`. Serwis i HTTP: walidacja (cel kcal 0–20000, makro 0–2000 g, ilość > 0 i ≤ 10000, jednostka `g`/`szt`, data poprawna, zakres ≤ 31 dni), izolacja użytkowników i gospodarstw (Review Focus 5), wyłączone śledzenie zwraca `enabled:false`, `TestRoutesMatchOpenAPI`.

- [ ] **Step 2–4:** fail → implementacja → pass.

- [ ] **Step 5: Commit** (`feat(nutrition): account goals, food log and daily summary`)

---

### Task 8: Frontend — typy, store'y, Konto i Produkty

**Files:**
- Modify: `web/src/api/types.ts`, `web/src/router.ts` (`/account` → `account`, `/products` → `products`), `web/src/layouts/AppShell.vue` (pozycje menu awatara „Konto” i „Produkty”), `web/src/messages.ts`
- Create: `web/src/stores/products.ts`, `web/src/stores/nutrition.ts`, `web/src/views/AccountView.vue`, `web/src/views/ProductsView.vue`, `web/src/components/ProductPicker.vue`, `web/src/nutrition.ts` (formatowanie)
- Test: `web/src/stores/products.test.ts`, `web/src/stores/nutrition.test.ts`, `web/src/nutrition.test.ts`

**Interfaces:**
- Produces (TS): `Macros {kcal: number|null; protein: number|null; carbs: number|null; fat: number|null}`, `Product`, `NutritionSettings {enabled: boolean; goal: Macros}`, `DaySummary`, `MealNutrition`, `LogNutrition`, `Source = 'recipe'|'computed'|'missing'`, `RecipeNutrition`; `useProducts()` (`load(q?)`, `save(p)`, `remove(id)`), `useNutrition()` (`loadSettings()`, `saveSettings(s)`, `loadDays(from,to)`, `addLog(input)`, `removeLog(id)`, stan `settings`, `days: Record<string, DaySummary>`); `formatMacros(m: Macros): string` i `formatRemaining(d: DaySummary): string` (np. „brakuje 640 kcal · 38 g białka”, „cel osiągnięty”) w `nutrition.ts`; `<ProductPicker v-model="productId" :allow-create="true" />` (autouzupełnianie po `GET /products?q=`, z opcją „Dodaj produkt”).

- [ ] **Step 1: Write the failing tests** — store'y wysyłają poprawne metody, ścieżki i ciała (wzorzec `mockFetch` z `stores/recipes.test.ts`); `formatMacros` zaokrągla kcal do 1, gramy do 1 miejsca po przecinku i pomija nil; `formatRemaining` dla celu nil, nadwyżki i braku.
- [ ] **Step 2–4:** fail → implementacja → pass (`.superpowers/bin/dknode npm test`, `npx vue-tsc --noEmit`).

AccountView: przełącznik „Śledź wartości odżywcze” i cztery pola celu (puste = brak celu). ProductsView: lista z wyszukiwaniem, dodawanie i edycja w dolnym arkuszu na telefonie / oknie na komputerze (pola: nazwa, kcal, białko, węglowodany, tłuszcz na 100 g, gramatura, gęstość, aliasy jako chipy), usuwanie z obsługą `in_use`.

- [ ] **Step 5: Commit** (`feat(web): account nutrition settings and product database`)

---

### Task 9: Frontend — wartości w edytorze przepisu

**Files:**
- Modify: `web/src/views/RecipeEditView.vue`, `web/src/stores/recipes.ts` (wysyła `nutrition` i `ingredients[].productId`), `web/src/importDraft.ts` (`draftToRecipe` i `mergeIntoExisting` przenoszą `nutrition`), `web/src/messages.ts`
- Test: `web/src/stores/recipes.test.ts`, `web/src/importDraft.test.ts`

**Interfaces:**
- Consumes: `ProductPicker`, `RecipeNutrition` (z `GET /recipes/{id}`), typy z Task 8.

- [ ] **Step 1: Write the failing tests** — `save` wysyła `nutrition` (nil zostaje null, pusty string z pola liczbowego → null) i `productId` (pusty → null); `draftToRecipe` przenosi `nutrition` szkicu; `mergeIntoExisting` zachowuje wartości istniejącego przepisu, gdy szkic ich nie ma.
- [ ] **Step 2–4:** fail → implementacja → pass.

Edytor: sekcja „Wartości na porcję” z czterema polami (podane lub ręczne); pole puste pokazuje jako podpowiedź wartość wyliczoną z dopiskiem „wyliczone” albo „brak”; ostrzeżenie „przybliżone: nie policzono …” z listą `unmatched`. Przy każdym składniku mały wybór produktu (`ProductPicker`), pokazujący dopasowany produkt; wybór zapisuje `productId`. Na telefonie wiersz składnika dalej się zawija (bez poziomego przewijania).

- [ ] **Step 5: Commit** (`feat(web): nutrition in the recipe editor`)

---

### Task 10: Frontend — podsumowanie dnia, posiłek i dziennik

**Files:**
- Create: `web/src/components/DaySummary.vue`, `MealNutritionDialog.vue`, `FoodLogDialog.vue`
- Modify: `web/src/views/TodayView.vue`, `web/src/views/PlanView.vue`, `web/src/messages.ts`
- Test: `web/src/nutrition.test.ts` (dalsze funkcje pomocnicze, jeśli powstaną)

**Interfaces:**
- Consumes: `useNutrition`, `formatMacros`, `formatRemaining`, `ProductPicker`.

- [ ] **Step 1: Write the failing tests** dla czystych funkcji pomocniczych, które wydzielisz (np. procent celu dla paska, tekst braków `MissingMeals` i `Approximate`).
- [ ] **Step 2–4:** fail → implementacja → pass.

Tylko dla konta ze śledzeniem. Dziś: pod listą posiłków `DaySummary` (kcal i trzy makro z paskami względem celu, tekst `formatRemaining`, ostrzeżenia „N posiłków bez wartości”, „wartości przybliżone”), lista dopisanych pozycji z usuwaniem i przyciskiem „Dopisz” otwierającym `FoodLogDialog` (produkt, ilość, `g`/`szt` gdy produkt ma gramaturę). Przy każdym posiłku chip z kcal porcji; kliknięcie chipa otwiera `MealNutritionDialog` (wartości jednej porcji i źródła). Plan tygodnia: pod każdym dniem jedna linia skrótu (kcal / cel, białko) z jednego `loadDays` na cały tydzień. Odświeżenie podsumowania po zmianie planu lub dziennika.

- [ ] **Step 5: Commit** (`feat(web): daily nutrition summary and food log`)

---

### Task 11: Skill i dokumentacja

**Files:**
- Modify: `web/public/claude-skill/SKILL.md`, `docs/import.md`, `docs/api.md`, `CHANGELOG.md`, `web/src/skill.test.ts`
- Create: `docs/nutrition.md`

- [ ] **Step 1: Write the failing test** — `skill.test.ts` dodatkowo sprawdza, że skill zawiera `Wartości na porcję:`.
- [ ] **Step 2: Update the skill** — format dostaje opcjonalną linię po `Porcje:`: `Wartości na porcję: 520 kcal, białko 32 g, węglowodany 60 g, tłuszcz 14 g`; zasady: tylko gdy źródło podaje wartości (z JSON-LD `nutrition` lub tekstu), zawsze na jedną porcję (przelicz, jeśli źródło podaje na 100 g lub na całość i zna wagę lub liczbę porcji), nigdy nie zgaduj; gdy brak, pomiń linię, aplikacja wyliczy sama. Zaktualizuj przykład i listę kontrolną.
- [ ] **Step 3: Docs** — `docs/nutrition.md`: skąd wartości (kolejność, per wartość), przeliczniki jednostek, dopasowanie i ręczny wybór produktu, dane startowe i licencja (USDA FoodData Central, CC0 1.0, generator), cele i dziennik, podsumowanie i znaczenie ostrzeżeń. `docs/import.md`: linia wartości i JSON-LD `nutrition`. `docs/api.md`: nowe trasy. `CHANGELOG.md`: sekcja `### Wartości odżywcze`.
- [ ] **Step 4: Verify** — `.superpowers/bin/dkgo go test -count=1 ./...`, `.superpowers/bin/dknode npm test`, `.superpowers/bin/dknode npm run build`, `touch web/dist/.gitkeep`.
- [ ] **Step 5: Commit** (`docs: nutrition, product data and skill format`)
