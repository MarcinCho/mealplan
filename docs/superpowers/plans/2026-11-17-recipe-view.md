# Widok przepisu i przeliczanie porcji Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Kliknięcie przepisu otwiera prosty widok do czytania przy gotowaniu, z przeliczaniem ilości składników na wybraną liczbę porcji; edycja przenosi się pod `recipes/:id/edit`.

**Architecture:** Tylko frontend. Czysty moduł `web/src/scale.ts` (przeliczanie i kuchenne zaokrąglanie, bez Vue), nowy widok `web/src/views/RecipeView.vue` pod trasą `recipe` (`recipes/:id`, porcje w `?servings=`), a trasa `recipe-edit` dostaje ścieżkę `recipes/:id/edit`. Linki z listy przepisów, ekranu Dziś i edytora prowadzą do widoku. Backend, API i baza bez zmian.

**Tech Stack:** Vue 3 + Vuetify 3 + Pinia + vue-router 4, Vitest + @vue/test-utils, TypeScript (`vue-tsc`).

**Spec:** `docs/superpowers/specs/2026-11-17-recipe-view-design.md` (zatwierdzony 2026-11-17).

## Global Constraints

- Gałąź `feat/recipe-view` z `master`. Commity kończą się linią `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Bez pushowania bez prośby.
- Host ma Go i Node: w `web/` `npm test` (vitest) i `npm run build` (`vue-tsc --noEmit && vite build`); po buildzie `web/dist/.gitkeep` musi zostać (przywróć `touch web/dist/.gitkeep`, jeśli build go usunie).
- Każdy tekst dla użytkownika jest w `web/src/locales/pl.ts` i ma odpowiednik w `en.ts` (TypeScript pilnuje kluczy, test pilnuje braku polskich liter w `en.ts`). Kod i komentarze po angielsku. Bez `v-html`.
- Telefon 390 px bez poziomego przewijania; kontrast tekstu drugorzędnego WCAG AA jak w reszcie aplikacji (używaj `text-medium-emphasis`, nie własnych kolorów).
- Porcje: liczby całkowite 1–100 (jak `maxServings` w `internal/recipe` i `internal/plan`).
- Współczynnik 1 pokazuje ilości dokładnie tak, jak są zapisane. Przeliczanie nigdy nie zapisuje niczego do API.
- Nazwa trasy `recipe-edit` zostaje (czyta ją `web/src/importDraft.ts`); `ImportView` dalej prowadzi do edycji przy duplikacie.

## Review Focus

- Przepis z `baseServings` innym niż 2 i `?servings` równym bazowym: brak „Przeliczone z…”, ilości bez zaokrągleń (test w Task 2).
- `?servings=abc`, `0`, `101`, `2.5`, podwójny parametr (`?servings=3&servings=5`): widok pokazuje porcje bazowe i nie wybucha (test w Task 1 dla `parseServings`, w Task 2 dla widoku).
- Składnik z jednostką spoza listy (np. `"cm"`) oraz z `quantity: 0`: nieznana zaokrąglana do 0,1 z minimum 0,1; zero zostaje zerem (test w Task 1).
- Przepis bez kroków, bez tagów, bez źródła i bez wartości odżywczych: puste sekcje nie zostawiają nagłówków (test w Task 2).
- Brak przepisu (404) i błąd sieci: komunikat zamiast pustej strony (test w Task 2).

---

### Task 1: Moduł przeliczania `scale.ts`

**Files:**
- Create: `web/src/scale.ts`
- Test: `web/src/scale.test.ts`

**Interfaces:**
- Produces:
  - `export const MAX_SERVINGS = 100`
  - `export function parseServings(raw: unknown, base: number): number` — `raw` to wartość z `route.query.servings` (string, tablica, null, undefined). Zwraca liczbę całkowitą 1–100 z pojedynczego stringa złożonego z samych cyfr; w każdym innym przypadku `base`.
  - `export function scaleAmount(quantity: number | null, unit: string, factor: number): { quantity: number | null; unit: string }` — `factor === 1` albo `quantity === null` zwraca wejście bez zmian; inaczej mnoży i zaokrągla wg tabeli §4.4 specu.

- [ ] **Step 1: Napisz testy**

`describe('scaleAmount')`, każdy przypadek jako `expect(scaleAmount(q, unit, f)).toEqual({ quantity, unit })`:
- współczynnik 1 nie zaokrągla: `(0.3, 'szklanka', 1)` → `0.3 szklanka`; `(333.33, 'g', 1)` → `333.33 g`
- `null` zostaje: `(null, '', 3)` → `null ''`
- zero zostaje zerem: `(0, 'g', 3)` → `0 g`
- przykłady ze specu (przepis na 2): `(125,'g',1.5)`→`190 g` (187,5 → do 10); `(125,'g',3)`→`380 g`; `(110,'g',3)`→`330 g`; `(4.5,'g',3)`→`14 g`; `(1,'łyżka',1.5)`→`1.5 łyżka`; `(1,'łyżeczka',0.25)`→`0.25 łyżeczka`; `(3,'',0.5)`→`1.5 ''`; `(1,'szczypta',0.5)`→`1 szczypta`; `(0.5,'kg',0.5)`→`250 g`
- pasma g/ml: `(3.3,'g',1.5)`→`5 g` (4,95 → do 0,5); `(0.1,'ml',2)`→`0.5 ml` (minimum); `(9.9,'g',1.01)`→`10 g`; `(998,'g',1.0001)`→`1 kg` (ponowne sprawdzenie pasma); `(800,'ml',2)`→`1.6 l`; `(1234,'g',2)`→`2.45 kg` (2468 g → 2,468 kg → do 0,05)
- kg/l: `(0.999,'kg',1.0001)`→`1 kg`; `(0.4,'l',2)`→`800 ml`; `(1.33,'kg',1.5)`→`2 kg` (1,995 → do 0,05 = 2)
- łyżki: `(1,'łyżka',0.4)`→`0.5 łyżka` (0,4 → do 0,25 = 0,5); `(1,'łyżeczka',0.1)`→`0.25 łyżeczka` (minimum); `(3,'szklanka',0.3)`→`1 szklanka` (0,9 → 1, na siatce 0,5 zostaje 1)
- sztuki i jednostki kuchenne: `(1,'szt',0.25)`→`0.5 szt` (minimum); `(0.5,'pęczek',2)`→`1 pęczek`; `(2,'ząbek',1.3)`→`2.5 ząbek`
- szczypta: `(1,'szczypta',3)`→`3 szczypta`; `(1,'szczypta',0.2)`→`1 szczypta`
- nieznana jednostka: `(3,'cm',0.33)`→`1 cm` (0,99 → do 0,1 = 1); `(0.1,'cm',0.1)`→`0.1 cm` (minimum); `(7,'cm',2)`→`14 cm`

`describe('parseServings')` z bazą 2: `'4'`→4, `'100'`→100, `'1'`→1; `undefined`, `null`, `''`, `'abc'`, `'0'`, `'101'`, `'2.5'`, `'-3'`, `' 4'`, `['3','5']` → 2.

- [ ] **Step 2: Uruchom i sprawdź, że padają**

Run: `cd web && npx vitest run src/scale.test.ts`
Expected: FAIL (`scale.ts` nie istnieje).

- [ ] **Step 3: Zaimplementuj `web/src/scale.ts`**

Zaokrąglanie do kroku: `Math.round(v / step) * step`, potem `Math.round(x * 1000) / 1000` (jak `units.Round`), żeby nie zostawały ogony typu `1.5000000002`. Zbiory jednostek: `SMALL = {g: 'kg', ml: 'l'}`, `BIG = {kg: 'g', l: 'ml'}`, `SPOONS = {łyżka, łyżeczka, szklanka}`, `PIECES = {szt, '', ząbek, plaster, opakowanie, puszka, pęczek, garść, kostka, główka, kromka}` (lista z `UNITS` w `locales/pl.ts` plus `szt` i brak jednostki). Algorytm dla przeliczonej wartości `v > 0`:

```
round(v, unit):
  g/ml:  if v >= 1000 → return round(v / 1000, SMALL[unit])
         r = v < 10 ? max(step(v, 0.5), 0.5) : v < 100 ? step(v, 1) : step(v, 10)
         return r >= 1000 ? { r / 1000, SMALL[unit] } : { r, unit }
  kg/l:  if v < 1 → return round(v * 1000, BIG[unit])
         return { step(v, 0.05), unit }
  SPOONS: { max(v < 1 ? step(v, 0.25) : step(v, 0.5), 0.25), unit }
  szczypta: { max(step(v, 1), 1), unit }
  PIECES: { max(step(v, 0.5), 0.5), unit }
  inne:  { max(v < 10 ? step(v, 0.1) : step(v, 1), 0.1), unit }
```

`v === 0` zwraca `{ 0, unit }` bez minimum.

- [ ] **Step 4: Uruchom i sprawdź, że przechodzą**

Run: `cd web && npx vitest run src/scale.test.ts`
Expected: PASS, wszystkie przypadki.

- [ ] **Step 5: Commit**

```bash
git add web/src/scale.ts web/src/scale.test.ts
git commit -m "feat(web): scale recipe amounts with kitchen rounding"
```

---

### Task 2: Widok przepisu `RecipeView.vue`

**Files:**
- Create: `web/src/views/RecipeView.vue`
- Modify: `web/src/locales/pl.ts`, `web/src/locales/en.ts` (nowe klucze)
- Test: `web/src/views/RecipeView.test.ts`

**Interfaces:**
- Consumes: `parseServings`, `scaleAmount`, `MAX_SERVINGS` z Task 1; `useRecipes().get(id)` (`stores/recipes.ts`); `formatQuantity(q, unit)` (`stores/recipes.ts`) do tekstu ilości; `useNutrition()` (`stores/nutrition.ts`: `settings`, `loadSettings()`); `formatNumber(v, 'kcal' | 'g')` (`nutrition.ts`); `portions(n)` (`polish.ts`); `errorText(e)`, `t` (`messages.ts`); `ApiError` (`api/client`) z `status === 404`.
- Produces: komponent trasy `recipe` (`/recipes/:id`), czyta `route.params.id` i `route.query.servings`, nawiguje do `{ name: 'recipe-edit', params: { id } }` i `{ name: 'recipes' }`. Task 3 rejestruje go w routerze.
- Nowe klucze w `pl.ts` (i odpowiedniki w `en.ts`): `servingsLabel: 'Porcje'`, `fewerServings: 'Mniej porcji'`, `moreServings: 'Więcej porcji'`, `scaledFrom: (n: number) => \`Przeliczone z ${n} ${portionWord(n)}\``, `restoreServings: 'Przywróć'`, `perPortion: 'na porcję'`, `approxPrefix: 'ok.'`, `source: 'Źródło'`, `editButton: 'Edytuj'`, `recipeNotFound: 'Nie znaleziono przepisu.'`. „Składniki”, „Kroki” i „Przepisy” już są (`t.ingredients`, `t.steps`, `t.recipes`). W `scaledFrom` dopełniacz: „z 2 porcji”, „z 1 porcji”, „z 5 porcji”, więc forma to zawsze „porcji” (w en: `Scaled from ${n} ${n === 1 ? 'serving' : 'servings'}`).

- [ ] **Step 1: Napisz testy**

Wzorzec montowania jak w `RecipeEditView.test.ts` (`vi.mock('../api/client')`, `fakeApi`, router `createMemoryHistory` z trasami `recipes`, `recipe` → `RecipeView`, `recipe-edit` → stub pod `/recipes/:id/edit`). Przepis testowy: Zupa ogórkowa z `baseServings: 2`, składniki `300 g ziemniak`, `3 '' ogórki kiszone` z notą `starte`, `1 łyżka masło`, `null '' sól i pieprz` z notą `do smaku`, 2 kroki, tagi `zupa`, `obiad`, `sourceUrl: 'https://example.com/zupa'`, `nutritionPerPortion: { values: { kcal: 320, protein: 12, carbs: 30, fat: 15 }, approximate: true, ... }`. `GET /me/nutrition` zwraca `{ enabled: true, ... }`.

- `shows the header`: tytuł `Zupa ogórkowa` w `h1`, tagi `zupa` i `obiad`, link powrotu z `href` `/recipes` i nazwą `t.recipes`.
- `shows ingredients one per line at base servings`: wiersze tekstem `300 g ziemniak`, `3 ogórki kiszone` z notą `starte`, `1 łyżka masło`, `sól i pieprz` z `do smaku`; brak tekstu `t.scaledFrom(2)`.
- `scales from the servings query`: `/recipes/r1?servings=4` → `600 g ziemniak`, `6 ogórki kiszone`, `2 łyżki masło`, `sól i pieprz` bez liczby, widoczny `Przeliczone z 2 porcji` i przycisk `Przywróć`.
- `plus and minus change amounts and the url`: klik `Więcej porcji` → `router.currentRoute.value.query.servings === '3'` i `450 g ziemniak`; klik `Mniej porcji` dwa razy → porcje 1, `150 g ziemniak`, `query.servings === '1'`; przy 1 przycisk `Mniej porcji` jest `disabled`; przy `?servings=100` `Więcej porcji` jest `disabled`.
- `restore drops the query`: z `?servings=4` klik `Przywróć` → brak `servings` w query, `300 g ziemniak`, brak `Przeliczone…`.
- `ignores an invalid servings query`: `?servings=0` i `?servings=abc` → porcje 2, brak `Przeliczone…`.
- `uses replace for servings changes`: po kliknięciu `+` `router.back()` wraca do trasy sprzed wejścia na widok (wejdź z `/recipes`, potem `/recipes/r1`, klik `+`, `router.back()` → `recipes`).
- `nutrition line`: `ok. 320 kcal na porcję` i wartości B/W/T; przy `GET /me/nutrition` → `{ enabled: false }` linii nie ma; przy `nutritionPerPortion` z samymi `null` linii nie ma.
- `hides empty sections`: przepis bez kroków, tagów i źródła → brak nagłówka `t.steps`, brak linku źródła.
- `source link`: `a[href="https://example.com/zupa"]` z `target="_blank"` i `rel="noopener noreferrer"`, tekst zawiera `example.com`.
- `edit button`: link do `/recipes/r1/edit` bez `servings` w query, nawet przy `?servings=4`.
- `not found`: `GET /recipes/r1` rzuca `new ApiError(404, 'not_found', '')` → widoczny `t.recipeNotFound` i link do listy; inny błąd → `errorText(e)`.

- [ ] **Step 2: Uruchom i sprawdź, że padają**

Run: `cd web && npx vitest run src/views/RecipeView.test.ts`
Expected: FAIL (`RecipeView.vue` nie istnieje).

- [ ] **Step 3: Dodaj klucze tłumaczeń i zaimplementuj `RecipeView.vue`**

Układ wg §4.2 specu, jedna kolumna z `max-width` ok. 720 px na komputerze. Porcje to `computed` z `parseServings(route.query.servings, recipe.baseServings)`; `+`/`−` i „Przywróć” wołają `router.replace({ query: n === base ? omit servings : { ...route.query, servings: String(n) } })`. Ilość składnika: `scaleAmount(i.quantity, i.unit, servings / base)`, tekst przez `formatQuantity`. Nota w `span.text-medium-emphasis`. Linia wartości odżywczych tylko gdy `nutrition.settings?.enabled === true` i jest choć jedna znana wartość; `loadSettings()` wołane, gdy `settings === null`, a jego błąd ukrywa linię (nie blokuje widoku). Przyciski porcji z `aria-label` (`t.fewerServings`/`t.moreServings`) i hit targetem ≥ 44 px. Domena źródła: `new URL(sourceUrl).hostname` bez `www.`, a przy błędnym URL cały tekst.

- [ ] **Step 4: Uruchom i sprawdź, że przechodzą**

Run: `cd web && npx vitest run src/views/RecipeView.test.ts src/i18n.test.ts src/messages.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/RecipeView.vue web/src/views/RecipeView.test.ts web/src/locales/pl.ts web/src/locales/en.ts
git commit -m "feat(web): recipe reading view with servings scaling"
```

---

### Task 3: Trasy, linki i dokumentacja

**Files:**
- Modify: `web/src/router.ts:39-43` (trasy przepisów)
- Modify: `web/src/views/RecipesView.vue:56` (link wiersza)
- Modify: `web/src/views/TodayView.vue:196-201` (przycisk „Przepis”)
- Modify: `web/src/views/RecipeEditView.vue:179` (po zapisie), `:279` (Anuluj)
- Modify: `web/src/views/RecipeEditView.test.ts` (router i oczekiwane trasy), `web/src/views/TodayView.test.ts` (trasa `recipe`)
- Create: `web/src/router.test.ts`
- Modify: `CHANGELOG.md` (sekcja „Unreleased”), `docs/roadmap.md` (punkt 2 zrobiony)

**Interfaces:**
- Consumes: `RecipeView.vue` z Task 2 (komponent trasy `recipe`).
- Produces: trasy `{ path: 'recipes/:id', name: 'recipe', component: RecipeView }` (lazy, jak inne) i `{ path: 'recipes/:id/edit', name: 'recipe-edit', component: RecipeEditView }`, zadeklarowane po `recipes/import`, `recipes/check`, `recipes/new`.

- [ ] **Step 1: Napisz/zmień testy**

- `RecipeEditView.test.ts`: router z trasami `recipes`, `recipe-new` (`/recipes/new`), `recipe` (`/recipes/:id`, stub), `recipe-edit` (`/recipes/:id/edit`); istniejące wejścia `show('/recipes/r1', …)` zmień na `/recipes/r1/edit`. Nowe/zmienione asercje: zapis istniejącego przepisu → `router.currentRoute.value` ma `name: 'recipe'`, `params.id: 'r1'` i komunikat `recipeSavedText`; zapis nowego → `name: 'recipe'` z `id` zwróconym przez `POST /recipes`; „Anuluj” przy istniejącym ma `href` `/recipes/r1`, przy nowym `/recipes`; usunięcie dalej → `recipes`.
- `TodayView.test.ts`: dodaj `recipe` do listy nazw tras; przycisk `t.openRecipe` przy posiłku z `recipeId: 'r1'` i `servings: 4` ma `href` zawierający `/recipe/r1` i `servings=4` (ścieżki w tym teście to `/${name}/:id?`).
- Nowy `web/src/router.test.ts` (import `{ router }` z `./router`; `resolve` nie uruchamia strażników): `router.resolve('/recipes/r1').name === 'recipe'`, `router.resolve('/recipes/r1/edit').name === 'recipe-edit'`, `router.resolve('/recipes/new').name === 'recipe-new'`, `router.resolve('/recipes/check').name === 'recipe-check'`, `router.resolve('/recipes/import').name === 'recipe-import'`.

- [ ] **Step 2: Uruchom i sprawdź, że padają**

Run: `cd web && npx vitest run src/views/RecipeEditView.test.ts src/views/TodayView.test.ts src/router.test.ts`
Expected: FAIL na nowych asercjach.

- [ ] **Step 3: Zmień trasy i linki**

Router jak w Interfaces. `RecipesView`: `:to="{ name: 'recipe', params: { id: r.id } }"`. `TodayView`: `:to="{ name: 'recipe', params: { id: hero.entry.recipeId }, query: { servings: String(hero.entry.servings) } }"`. `RecipeEditView`: po zapisie `router.push({ name: 'recipe', params: { id: saved.id } })`; „Anuluj”: `isNew ? { name: 'recipes' } : { name: 'recipe', params: { id: route.params.id } }`. `ImportView` bez zmian.

- [ ] **Step 4: Dokumentacja**

`CHANGELOG.md`, w „Unreleased” nowa sekcja `### Widok przepisu` (pierwsza): kliknięcie przepisu otwiera widok do czytania (składnik w jednej linii, ponumerowane kroki, kalorie na porcję, link do źródła); przeliczanie porcji przyciskami − i + z kuchennym zaokrąglaniem, bez zmiany przepisu; „Przepis” na ekranie Dziś otwiera widok na liczbie porcji z planu; edycja pod `/recipes/{id}/edit`, a zapis wraca do widoku przepisu. `docs/roadmap.md`: w tabeli punkt 2 z dopiskiem „— **zrobione**”, nagłówek sekcji `## 2. … (zrobione)`, a w „Do decyzji” odpowiedź: przeliczona liczba porcji się nie zapisuje, porcje bazowe zmienia się w „Edytuj”.

- [ ] **Step 5: Pełna weryfikacja**

Run: `cd web && npm test && npm run build`
Expected: wszystkie testy PASS, `vue-tsc` bez błędów, build OK; `web/dist/.gitkeep` istnieje.
Run (z katalogu repo): `go test ./docs/...`
Expected: PASS (linki względne i bloki mermaid w dokumentacji).

- [ ] **Step 6: Commit**

```bash
git add web/src/router.ts web/src/router.test.ts web/src/views/RecipesView.vue web/src/views/TodayView.vue web/src/views/RecipeEditView.vue web/src/views/RecipeEditView.test.ts web/src/views/TodayView.test.ts CHANGELOG.md docs/roadmap.md
git commit -m "feat(web): open recipes in the reading view, edit under /edit"
```
