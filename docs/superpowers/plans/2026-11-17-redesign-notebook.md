# Nowy wygląd „notes” Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cała aplikacja dostaje wygląd „spokojnie jak notes” (czcionki Red Hat, klawisze 6 px, karty 10 px, liniowane wiersze, górny pasek z linkami), a ekrany Dziś, Przepis i Plan dostają układ z makiet, bez utraty żadnej dzisiejszej akcji.

**Architecture:** Tylko frontend (`web/`). System wyglądu to trzy miejsca: kolory i czcionki w motywach Vuetify (`theme.ts`, nowa para `note`/`on-note`, zmienne `font-text`/`font-display`), domyślne warianty w `plugins/vuetify.ts` i jeden globalny plik `web/src/styles/notebook.css` (kształty, czcionki, linie, kratka). Dwa prezentacyjne komponenty notesu (`NoteCard`, `NoteRow`) i mały `CookBadge` w `web/src/components/notebook/` składają trzy przebudowane ekrany. Logika widoków, store'y i API bez zmian.

**Tech Stack:** Vue 3 + Vuetify 3 (3.13) + Pinia + vue-router 4, Vitest + @vue/test-utils + jsdom, TypeScript (`vue-tsc`), `@fontsource/red-hat-display` i `@fontsource/red-hat-text`.

**Spec:** `docs/superpowers/specs/2026-11-17-redesign-notebook-design.md`. Wzorzec wizualny: `docs/design/2026-11-redesign/` (zrzuty w `screenshots/`, makiety `dzis.html`, `przepis.html`, `plan.html`, `app.css`, `tokens.css`).

## Global Constraints

- Gałąź `feat/redesign-notebook` z `master` (6a08afa). Commity: `git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit -m "<tytuł>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"` (host nie ma tożsamości git; nigdy nie ustawiaj `git config`). Każdy commit kończy się linią `Co-Authored-By`. Bez pushowania bez prośby.
- Komendy z `web/`: `npx vitest run <pliki>` (wybrane), `npx vitest run` (całość, ok. 2 min; kilka ciężkich testów montowania potrafi przekroczyć czas pod obciążeniem — takie pliki uruchom ponownie osobno), `npm run build` (`vue-tsc --noEmit && vite build`). Po buildzie `web/dist/.gitkeep` musi istnieć (`git restore web/dist/.gitkeep`); nigdy nie commituj wyniku buildu. Z katalogu repo `go test ./docs/`. `go test ./...` ma dwa istniejące błędy CRLF na tym hoście — ignoruj.
- Nowe zależności npm tylko: `@fontsource/red-hat-display@5.3.0` i `@fontsource/red-hat-text@5.3.0` (dokładne wersje, `dependencies`). Vuetify zostaje 3.x. Bez Google Fonts i innych zewnętrznych czcionek/CDN; tylko podzbiory `latin` i `latin-ext`, tylko używane grubości (Text 400, 500, 700; Display 600, 700).
- Każdy tekst dla użytkownika w `web/src/locales/pl.ts` z odpowiednikiem w `en.ts` (test pilnuje braku polskich liter w `en.ts`). Kod i komentarze po angielsku. Bez `v-html`.
- Telefon 390 px bez poziomego przewijania; tekst drugorzędny przez `text-medium-emphasis`, nie własne kolory.
- Próg telefon/komputer wszędzie `mdAndUp` z Vuetify (960 px): nawigacja, panel boczny Dziś, dwie kolumny i przyklejona karta w Przepisie, tabela w Planie.
- Kształty: przyciski (poza ikonowymi, które zostają okrągłe) 6 px, bez wersalików; karty 10 px bez ramki i cienia, tło `surface`; pola obrysowane 6 px; chipy i alerty 6 px; listy bez zaokrągleń; menu, snackbar i dialogi 10 px.
- Warianty ustawione jawnie w szablonach poza Dziś, Przepisem, Planem i `AppShell` zostają (np. `variant="tonal"`), zmienia się tylko kształt. Na trzech ekranach: główna akcja wypełniona `primary`, reszta obrysowana lub tekstowa.
- Kolory notesu tylko z palety: tło `background`, karty i wiersze `surface`, kratka i wyróżniony wiersz `surface-variant`, nagłówek karty `note`/`on-note`, główna akcja i „Dziś” `primary`/`on-primary`, odhaczone `success`, linie jako przezroczysta odmiana `on-surface`.
- Przełącznik palety i trybu zostaje w menu konta. 5 palet × jasny/ciemny zostaje.
- Bez zmian w API, danych, store'ach i logice komponentów; teksty bez zmian poza miejscami, gdzie układ rozdziela sklejony tekst (np. „Obiad · 4 porcje” → kolumna pory + linia z porcjami).
- Istniejące testy widoków przechodzą. Zmieniać je wolno tylko tam, gdzie opierają się na znacznikach Vuetify, których już nie ma, i zawsze z zachowaniem tego, co sprawdzają. Nowe testy zachowań piszemy tak, żeby nie zależały od układu (nazwy dostępne, `title`, `aria-*`, tekst).

## Review Focus

- Bardzo długa nazwa gospodarstwa (100 znaków) na komputerze ≥ 1280 px: nazwa przycięta wielokropkiem, linki w jednej linii, pełna nazwa w `title` (test w Task 4).
- Wpis z kucharzem, którego nie ma już na liście domowników (`cookUserId: 'gone'`): Dziś i Plan renderują wpis bez znacznika osoby i bez tekstu `undefined` (testy w Task 6 i Task 8).
- Odhaczanie posiłku i kliknięcie znacznika kalorii nie otwierają edycji wpisu, a tytuł posiłku/wpisu jest przyciskiem osiągalnym z klawiatury (testy w Task 6 i Task 8).
- Dwa wpisy w tej samej porze tego samego dnia: oba widoczne i oba otwierają swoją edycję, w tabeli i na telefonie (test w Task 8).
- Bardzo długi tytuł przepisu/posiłku bez spacji przy 390 px: brak poziomego przewijania (sprawdzenie `scrollWidth` w skrypcie zrzutów w Task 9, z takim wpisem w danych).

---

### Task 1: Testy zachowań Planu na starym układzie

Kolejność za specem §4.8: `PlanView.vue` nie ma testów, więc najpierw przypinamy zachowanie, potem dopiero ruszamy wygląd (Task 8 musi przejść te same testy bez zmian asercji).

**Files:**
- Create: `web/src/views/PlanView.test.ts`
- Modify: `web/src/test/mount.ts` (helper szerokości ekranu)
- Commit także: `docs/superpowers/specs/2026-11-17-redesign-notebook-design.md` i ten plan (dziś nieśledzone)

**Interfaces:**
- Produces: `export function setViewportWidth(px: number): void` w `web/src/test/mount.ts` — ustawia `window.innerWidth` (przez `Object.defineProperty(window, 'innerWidth', { value: px, configurable: true, writable: true })`). Działa, bo `mountWith` tworzy nowy Vuetify przy każdym montowaniu, a `useDisplay` czyta `window.innerWidth` przy tworzeniu. Pliki, które go używają, w `afterEach` wracają do `setViewportWidth(1024)` (domyślna szerokość jsdom). Używany w Task 4, 6, 7, 8.
- Produces: `export function clickable(text: string): HTMLElement` w `web/src/test/mount.ts` — spośród `button, a, [tabindex]` w `document` bierze elementy, których `textContent` zawiera `text`, i zwraca ten, który nie zawiera żadnego innego trafienia (najgłębszy); rzuca `Error`, gdy brak. Niezależny od układu (stary `v-chip`, `v-list-item` z `tabindex="-2"`, nowy `button`). Oraz `export function openDialog(): HTMLElement | null` = `document.querySelector('.v-overlay--active')`. Używane w Task 6 i 8.
- Kontrakt dla Task 8 (wyłącznie to, na czym opierają się testy): przyciski poprzedni/następny mają `title` `t.prevWeek`/`t.nextWeek`; `h1` z zakresem tygodnia (`formatDay(pon) – formatDay(nd)`); przyciski tekstowe `t.thisWeek`, `t.copyPrevWeek`, `t.retry`; element `.today-tag` z tekstem `t.today`, którego rodzic zawiera `formatDay(dziś)`; na komputerze przycisk dodawania w każdej komórce z `aria-label` `${t.addEntryTitle}: ${formatDay(d)}, ${slot.title}`; na telefonie 7 przycisków, których nazwa (`aria-label` albo tekst) zaczyna się od `t.addEntryTitle`; ostrzeżenie kalorii jako element z `aria-label` równym tekstowi ostrzeżenia.

- [ ] **Step 1: Gałąź i commit dokumentów**

```bash
git switch -c feat/redesign-notebook master
git add docs/superpowers/specs/2026-11-17-redesign-notebook-design.md docs/superpowers/plans/2026-11-17-redesign-notebook.md
git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit -m "docs: add the notebook redesign spec and plan" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 2: Napisz testy**

Wzorzec montowania jak w `TodayView.test.ts` (`api.current` + `vi.mock('../api/client')` z `ApiError` jak w `RecipeView.test.ts`, `vi.useFakeTimers({ toFake: ['Date'] })`, dziś `2026-11-11T09:00`, tydzień `2026-11-09`, router z nazwanymi trasami-stubami). Dane:
- przepisy `r1` Gulasz, `r2` Zupa ogórkowa; domownik `u1` Marc;
- wpisy tygodnia `2026-11-09`: `p1` 09.11 obiad `r1`, 4 porcje, `cookUserId: 'u1'`, `kind: 'meal'`; `p2` 11.11 kolacja `r2`, 2 porcje, `kind: 'prep'`, `prepDishId: 'd1'`; `p3` 12.11 obiad, `kind: 'out'`, `freeText: 'Pizza'`;
- `GET /plan/2026-11-16` i `GET /plan/2026-11-02` → puste; `GET /me/nutrition` → `{ enabled: false }`; `GET /prep/sessions`, `GET /freezer`, `GET /household/settings` jak w `TodayView.test.ts`.

Helpery `clickable(text)` i `openDialog()` z `test/mount.ts` (Interfaces); „dialog zawiera X” = `openDialog()?.textContent` zawiera X.

Wszystko w `describe.each([['desktop', 1280], ['phone', 390]])`:
- `switches weeks and returns to this week`: `h1` zawiera `formatDay('2026-11-09')` i `formatDay('2026-11-15')`; brak przycisku `t.thisWeek`; klik `button[title="${t.nextWeek}"]` → w `calls` jest `GET /plan/2026-11-16`, `h1` zawiera `formatDay('2026-11-16')`, jest `t.thisWeek`; klik `t.thisWeek` → `h1` znów z `formatDay('2026-11-09')`, przycisku brak; klik `button[title="${t.prevWeek}"]` → `GET /plan/2026-11-02`.
- `copies the previous week into an empty week`: następny tydzień, klik `t.copyPrevWeek` → `POST /plan/2026-11-16/copy` z body `{ fromWeek: '2026-11-09' }`; tekst zawiera `t.weekCopied`.
- `blocks copying until the week loads and retries after an error`: pierwszy `GET /plan/2026-11-09` rzuca `new Error('boom')` → tekst zawiera `errorText(err)`, `buttonByText(t.copyPrevWeek).disabled === true`; klik `t.retry` (drugie wywołanie zwraca wpisy) → `disabled === false`, brak błędu, widać `Gulasz`.
- `marks today`: dokładnie jeden `.today-tag` z tekstem `t.today`, `tag.parentElement.textContent` zawiera `formatDay('2026-11-11')`; po `t.nextWeek` brak `.today-tag`.
- `adds a meal with a screen reader description`: desktop — 28 przycisków `aria-label` = `${t.addEntryTitle}: ${formatDay(d)}, ${s.title}` dla 7 dni × `SLOTS`; klik tego dla (`2026-11-12`, `breakfast`) → dialog zawiera `t.addEntryTitle` i `t.slotNames.breakfast`. Phone — dokładnie 7 przycisków, których `aria-label ?? textContent.trim()` zaczyna się od `t.addEntryTitle`; klik czwartego → dialog zawiera `t.addEntryTitle`, `formatDay('2026-11-12')` i `t.slotNames.dinner`.
- `opens an entry for editing on click`: `clickable('Gulasz').click()` → dialog zawiera `t.editEntryTitle`.
- `shows the cook and the entry note`: tekst zawiera `Marc`, `t.chipPrep` i `t.chipOut`, oraz `Pizza`.
- `shows the day's calories with a warning`: `GET /me/nutrition` → `enabled: true`, cel 2200 kcal; `GET /me/nutrition/days?from=2026-11-09&to=2026-11-15` → dzień `2026-11-09` (1840 kcal, `missingMeals: 1`) i `2026-11-10` (0 kcal, bez wpisów; pola zgodne z `DaySummary` w `api/types.ts`) → tekst zawiera `dayLine(day0911)`, nie zawiera `dayLine(day1011)`; istnieje `[aria-label="${t.mealsWithoutValues(1)}"]`.

- [ ] **Step 3: Uruchom na starym układzie**

Run: `cd web && npx vitest run src/views/PlanView.test.ts`
Expected: PASS (16 testów). Jeśli któryś pada, poprawiasz test (selektor/fixture), nie `PlanView.vue`.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/PlanView.test.ts web/src/test/mount.ts
git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit -m "test(web): pin plan view behaviour before the redesign" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Palety — `note`/`on-note`, `surface-variant` i kontrast

**Files:**
- Modify: `web/src/theme.ts` (`define()` i 10 definicji kolorów)
- Test: `web/src/theme.test.ts`

**Interfaces:**
- Produces (kolory każdego z 10 motywów w `VUETIFY_THEMES`): `surface-variant`, `on-surface-variant`, `note`, `on-note` (hex `#RRGGBB`). Vuetify z nich generuje `--v-theme-note`, `--v-theme-on-note` i klasę `bg-note` (używa jej `NoteCard` w Task 5).
- `define()` ustawia domyślnie `'on-surface-variant'` na `colors['on-surface'] ?? (dark ? '#FFFFFF' : '#000000')`, chyba że paleta poda własne. Powód: Vuetify scala każdy motyw z wbudowanym, więc dziś każda paleta dziedziczy `on-surface-variant` `#EEEEEE` (jasny) albo `#000000` (ciemny), co na jasnej kratce daje nieczytelny tekst i podpowiedzi.
- Wartości: `mealplan` jasny `note: '#233831'`, `on-note: '#EEF4EC'` (jak w specu). Pozostałe dobiera wykonawca wg zasady §4.3 (jasny tryb: ciemny pas; ciemny tryb: jasny pas). Sprawdzone propozycje, które przechodzą wszystkie progi: mealplan ciemny `#C9DCBE`/`#141A17`; classic jasny `surface-variant #ECEFF1`, `note #37474F`/`#FFFFFF`; classic ciemny `surface-variant #2C2C2C`, `note #CFD8DC`/`#1A2327`; plum jasny `#290044`/`#EEE3B0`, ciemny `#EEE3B0`/`#290044`; earth jasny `#09251C`/`#F3D2AF`, ciemny `#F3D2AF`/`#09251C`; navy jasny `#27374D`/`#DDE6ED`, ciemny `#DDE6ED`/`#27374D`.

- [ ] **Step 1: Napisz testy**

W `describe('palettes')` helper `fg(c, key, dark)` = `c[key] ?? (dark ? '#FFFFFF' : '#000000')` (ta sama reguła co w istniejącym teście medium-emphasis):
- `registers ten themes`: `Object.keys(VUETIFY_THEMES)` ma długość 10.
- `defines the notebook colors in every theme`: dla każdego motywu `surface-variant`, `on-surface-variant`, `note`, `on-note` pasują do `/^#[0-9A-Fa-f]{6}$/`.
- `gives classic its own surface-variant`: `VUETIFY_THEMES.light.colors['surface-variant']` ≠ `'#424242'`, `VUETIFY_THEMES.dark.colors['surface-variant']` ≠ `'#c8c8c8'` (wartości wbudowane Vuetify).
- `keeps the Mealplan note header dark green`: `VUETIFY_THEMES['mealplan-light'].colors.note === '#233831'`.
- `keeps notebook text readable (WCAG AA)`: dla każdego motywu `contrast(fg(c,'on-surface'), c.surface) >= 4.5`, `contrast(fg(c,'on-surface'), c.background) >= 4.5`, `contrast(fg(c,'on-surface'), c['surface-variant']) >= 4.5`, `contrast(c['on-surface-variant'], c['surface-variant']) >= 4.5`, `contrast(c['on-note'], c.note) >= 4.5`, `contrast(c['on-primary'], c.primary) >= 4.5`.
- `sets interface elements apart (3:1)`: `contrast(c.note, c.surface) >= 3`, `contrast(c.success, c.surface) >= 3`.
- `keeps the grid tint visible but quiet`: `contrast(c['surface-variant'], c.surface)` w przedziale `[1.1, 1.6]`.

- [ ] **Step 2: Uruchom i sprawdź, że padają**

Run: `cd web && npx vitest run src/theme.test.ts`
Expected: FAIL (brak `note`, `on-note`; classic bez `surface-variant`).

- [ ] **Step 3: Uzupełnij palety i `define()` w `web/src/theme.ts`**

Wartości jak w Interfaces; `surface-variant` w innych paletach zmieniaj tylko, gdy test kratki pada. Komentarz przy `note` po angielsku: nagłówek karty notesu, zawsze odcięty od karty.

- [ ] **Step 4: Uruchom i sprawdź, że przechodzą**

Run: `cd web && npx vitest run src/theme.test.ts src/brand.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/theme.ts web/src/theme.test.ts
git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit -m "feat(web): add note colors and explicit surface variants to every palette" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Czcionki, domyślne warianty i globalne style notesu

**Files:**
- Modify: `web/package.json`, `web/package-lock.json` (`npm install --save-exact @fontsource/red-hat-display@5.3.0 @fontsource/red-hat-text@5.3.0`)
- Create: `web/src/styles/notebook.css`
- Create: `web/src/styles/notebook.test.ts`
- Create: `web/src/plugins/vuetify.test.ts`
- Modify: `web/src/main.ts` (importy czcionek i `notebook.css` po `vuetify/styles`)
- Modify: `web/src/theme.ts` (`FONT_TEXT`, `FONT_DISPLAY`, zmienne motywu), `web/src/theme.test.ts`
- Modify: `web/src/plugins/vuetify.ts` (`defaults`), `web/src/App.vue` (waga przycisków)
- Modify: `web/src/layouts/AppShell.vue:68,75,85-90,128` (usuń `rounded="xl"`, `rounded="lg"`, `rounded="pill"`), `web/src/components/LanguageSwitch.vue:14` (usuń `rounded="pill"`)

**Interfaces:**
- Produces w `theme.ts`: `export const FONT_TEXT = "'Red Hat Text', ui-sans-serif, system-ui, sans-serif"`, `export const FONT_DISPLAY = "'Red Hat Display', 'Red Hat Text', ui-sans-serif, sans-serif"`; `define()` dodaje do `variables` klucze `'font-text': FONT_TEXT` i `'font-display': FONT_DISPLAY` (Vuetify emituje je jako `--v-font-text` i `--v-font-display` na `:root` i w klasach motywów, więc działają też w dialogach poza `v-app`).
- Produces w `notebook.css` (używane w Task 4–8):
  - zmienne `:root`: `--mp-radius-control: 6px`, `--mp-radius-card: 10px`, `--mp-line` (cienka linia: `rgba(var(--v-theme-on-surface), 0.14)`), `--mp-rule` (gruba linia pod nagłówkiem sekcji: `rgb(var(--v-theme-on-surface))`), `--mp-content-max: 1152px`;
  - klasy: `.mp-grid` (tło `surface-variant` z kratką rysowaną gradientem w `rgba(var(--v-theme-on-surface), …)`), `.mp-ruled` (linia `--mp-line` u dołu), `.mp-num` (`font-variant-numeric: tabular-nums`), `.mp-display` (`font-family: var(--v-font-display)`, grubość 600);
  - czcionki: `html` i `.v-application` → `font-family: var(--v-font-text)`; `h1, h2, h3`, `.text-h1`…`.text-h6` → `var(--v-font-display)`; `.text-subtitle-*`, `.text-body-*`, `.text-button`, `.text-caption`, `.text-overline` → `var(--v-font-text)`. Klasy typografii Vuetify mają `font-family: "Roboto" !important`, więc nadpisanie też z `!important` i wyższą specyficznością (np. prefiks `:root`);
  - kształty: `.v-btn:not(.v-btn--icon)` i `.v-btn-group` → `--mp-radius-control`; `.v-card` → `--mp-radius-card`; `.v-field` (obrys pól), `.v-chip`, `.v-alert` → `--mp-radius-control`; `.v-menu > .v-overlay__content`, `.v-snackbar__wrapper`, `.v-dialog > .v-overlay__content` (i ich `.v-card`) → `--mp-radius-card`; `.v-list` → 0; listy na tle strony (`.v-main .v-list:not(.v-card .v-list) > .v-list-item + .v-list-item`) dostają górną linię `--mp-line`;
  - drugorzędny klawisz: `.v-btn--variant-flat:not(.v-btn--icon):not([class*='bg-'])` (płaski przycisk bez koloru) dostaje obrys 1 px `rgba(var(--v-theme-on-surface), 0.24)` — tak wygląda „Przepis” w makiecie, a przyciski `color="primary"` zostają wypełnione.
- `vuetify.ts` `defaults` (bez żadnego `rounded` i bez `border` na `VCard`): `VBtn: { variant: 'flat' }`, `VCard: { variant: 'flat' }`, `VTextField`/`VSelect`/`VAutocomplete`/`VCombobox: { variant: 'outlined', density: 'comfortable' }`, `VTextarea: { variant: 'outlined' }`, `VSwitch: { inset: true, color: 'primary' }`. Komentarz nad `defaults` zmienia się z „Material 3 feel” na notatnik (kształty w `styles/notebook.css`).
- Importy w `main.ts` (po `import 'vuetify/styles'`, przed `./styles/notebook.css`): `@fontsource/red-hat-text/latin-{400,500,700}.css`, `@fontsource/red-hat-text/latin-ext-{400,500,700}.css`, `@fontsource/red-hat-display/latin-{600,700}.css`, `@fontsource/red-hat-display/latin-ext-{600,700}.css` (10 importów; nazwy plików fontsource v5 — po instalacji sprawdź `ls node_modules/@fontsource/red-hat-text/latin-ext-400.css`).
- `App.vue`: reguła `.v-btn.v-btn` zostaje (bez wersalików), `font-weight: 600` → `500`.

- [ ] **Step 1: Zainstaluj czcionki**

Run: `cd web && npm install --save-exact @fontsource/red-hat-display@5.3.0 @fontsource/red-hat-text@5.3.0`
Expected: w `package.json` `"@fontsource/red-hat-display": "5.3.0"` i `"@fontsource/red-hat-text": "5.3.0"`; pliki `latin-ext-400.css` (Text) i `latin-ext-600.css` (Display) istnieją.

- [ ] **Step 2: Napisz testy**

`web/src/theme.test.ts`:
- `uses Red Hat Text for body text and Red Hat Display for headings in every theme`: dla każdego motywu `String(def.variables['font-text'])` zaczyna się od `'Red Hat Text',`, `String(def.variables['font-display'])` od `'Red Hat Display',`.

`web/src/plugins/vuetify.test.ts` (import `{ vuetify }` z `./vuetify`, czyta `vuetify.defaults.value`):
- `drops the Material 3 pills and borders`: `JSON.stringify(defaults)` nie zawiera `"rounded"`; `defaults.VCard.border` jest `undefined`; `defaults.VBtn.variant === 'flat'`; `defaults.VTextField.variant === 'outlined'`.

`web/src/styles/notebook.test.ts` (importy `?raw`: `./notebook.css?raw`, `../main.ts?raw`, `../../index.html?raw`):
- `sets body and heading fonts from the theme`: CSS pasuje do `/html[^{]*\{[^}]*font-family:\s*var\(--v-font-text\)/` i dla każdego `n` 1–6 zawiera `.text-h${n}`; zawiera `var(--v-font-display)`.
- `defines the notebook shapes`: zawiera `--mp-radius-control: 6px` i `--mp-radius-card: 10px` oraz selektory `.v-btn`, `.v-card`, `.v-field`, `.v-chip`, `.v-alert`, `.v-menu > .v-overlay__content`, `.v-snackbar__wrapper`, `.v-dialog > .v-overlay__content`.
- `bundles the fonts and never loads them from another server`: `main.ts` zawiera każdą z 10 ścieżek importu z Interfaces; połączony tekst CSS + `main.ts` + `index.html` nie pasuje do `/fonts\.googleapis|fonts\.gstatic|@import\s+url\(\s*['"]?https?:/`.

- [ ] **Step 3: Uruchom i sprawdź, że padają**

Run: `cd web && npx vitest run src/theme.test.ts src/plugins/vuetify.test.ts src/styles/notebook.test.ts`
Expected: FAIL (brak zmiennych czcionek, `rounded` w defaults, brak `notebook.css`).

- [ ] **Step 4: Zaimplementuj**

Jak w Interfaces. Wartości wysokości, odstępów, grubości linii i rozmiarów pisma według `docs/design/2026-11-redesign/app.css` i `tokens.css` (np. `letter-spacing` nagłówków ok. `-0.02em`, interlinia treści 1.5).

- [ ] **Step 5: Uruchom testy i build**

Run: `cd web && npx vitest run src/theme.test.ts src/plugins/vuetify.test.ts src/styles/notebook.test.ts src/layouts/AppShell.test.ts`
Expected: PASS.
Run: `cd web && npm run build && ls dist/assets | grep -c "red-hat" && grep -rl "fonts.googleapis" dist; git restore dist/.gitkeep`
Expected: build OK, liczba plików `red-hat*` > 0 (woff2 dla latin i latin-ext), `grep` niczego nie znajduje; `web/dist/.gitkeep` istnieje.

- [ ] **Step 6: Commit**

```bash
git add web/package.json web/package-lock.json web/src/main.ts web/src/App.vue web/src/theme.ts web/src/theme.test.ts web/src/plugins/vuetify.ts web/src/plugins/vuetify.test.ts web/src/styles/notebook.css web/src/styles/notebook.test.ts web/src/layouts/AppShell.vue web/src/components/LanguageSwitch.vue
git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit -m "feat(web): notebook fonts, shapes and component defaults" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Nawigacja w `AppShell`

**Files:**
- Modify: `web/src/layouts/AppShell.vue`
- Test: `web/src/layouts/AppShell.test.ts`

**Interfaces:**
- Consumes: `setViewportWidth` (Task 1); `--mp-line`, `--mp-content-max`, `bg-surface-variant` (Task 2–3).
- Produces (struktura, na której opierają się testy):
  - komputer (`mdAndUp`): `v-app-bar` płaski (bez `scroll-behavior="elevate"`), tło `background`, linia `--mp-line` u dołu; w nim `BrandLogo`, `<nav :aria-label="t.navMain" class="top-nav">` z 6 linkami `<a class="nav-link">` (kolejność `items` bez zmian), po prawej `span.household-name` (gdy `lgAndUp`) i menu konta. Aktywny link: tło `surface-variant`. Bez `v-navigation-drawer`.
  - telefon: `v-bottom-navigation tag="nav" :aria-label="t.navMain"` z 6 linkami `a.nav-item` (ikona + podpis, jak dziś); logo jako sam znak. Wysokość paska zostaje 72 px (od niej zależą `.quick-dock` w Dziś i `.notice-phone`).
  - aktywny link w obu wariantach ma `aria-current="page"` (reguła aktywności bez zmian: `home` — `isExactActive`, reszta — `isActive`).
  - nazwa gospodarstwa: przy `lgAndUp` (≥ 1280 px) w pasku jako `span.household-name` z `:title` = pełna nazwa, `max-width` z wielokropkiem; poniżej `lg` (także telefon) pierwszy element menu konta to `v-list-subheader.household-name` z nazwą. Nigdy oba naraz, nigdy żaden. *(decyzja: przy 960–1279 px pasek nie mieści nazwy obok 6 linków)*
  - `v-container` treści: `max-width: var(--mp-content-max)`, wyśrodkowany. Menu konta i snackbar bez `rounded` (kształt z Task 3).

- [ ] **Step 1: Napisz testy**

Helper w pliku: `mountShell(path: string, width: number)` (jak istniejący test: `useSession().$patch` z gospodarstwem `Dom`, router z płaskimi trasami, `setViewportWidth(width)` przed montowaniem). `afterEach`: `setViewportWidth(1024)`. `NAV = [t.navToday, t.navPlan, t.navPrep, t.navRecipes, t.navPantry, t.navShopping]`.
- `shows six text links in the top bar on desktop`: 1440 → `document.querySelectorAll(\`nav[aria-label="${t.navMain}"]\`)` ma długość 1; teksty jego `a` (trim) = `NAV`; `.v-navigation-drawer` nie istnieje.
- `shows six tabs at the bottom on a phone`: 390 → jedna `nav[aria-label]`, wewnątrz `.v-bottom-navigation`, teksty 6 linków = `NAV`.
- `marks the active link with aria-current` (desktop i telefon): na `/plan` dokładnie jeden `a[aria-current="page"]` z tekstem `t.navPlan`; na `/` → `t.navToday`; na `/recipes` → `t.navRecipes`.
- `shows the household name in the bar on wide screens`: 1440 → `.v-app-bar .household-name` ma tekst `Dom` i `title` `Dom`; po otwarciu menu (`button[title="${t.account}"]`) `.v-overlay--active .household-name` nie istnieje.
- `moves the household name into the account menu on narrower screens`: dla 1000 i 390 → brak `.v-app-bar .household-name`; po otwarciu menu `.v-overlay--active .household-name` ma tekst `Dom`.
- `keeps a long household name whole in the title` (Review Focus): nazwa = `'D'.repeat(100)`, 1440 → `.household-name` ma `title` o długości 100.
- Istniejący `closes the account menu when one of its links opens a page` bez zmian.

- [ ] **Step 2: Uruchom i sprawdź, że padają**

Run: `cd web && npx vitest run src/layouts/AppShell.test.ts`
Expected: FAIL (dziś jest `v-navigation-drawer`, brak `aria-current`, nazwa tylko w pasku).

- [ ] **Step 3: Zaimplementuj nawigację w `AppShell.vue`**

Układ wg górnego paska z `screenshots/dzis-desktop.png` (linki tekstowe, aktywny na tle `surface-variant`, 6 px) i dolnego paska z `screenshots/dzis-phone.png` (z ikonami, jak dziś). Styl `.rail*` usuń.

- [ ] **Step 4: Uruchom i sprawdź, że przechodzą**

Run: `cd web && npx vitest run src/layouts/AppShell.test.ts`
Expected: PASS (7 testów).

- [ ] **Step 5: Commit**

```bash
git add web/src/layouts/AppShell.vue web/src/layouts/AppShell.test.ts
git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit -m "feat(web): top bar navigation on desktop, notebook bottom bar on phones" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Komponenty notesu `NoteCard`, `NoteRow`, `CookBadge`

**Files:**
- Create: `web/src/components/notebook/NoteCard.vue`, `NoteRow.vue`, `CookBadge.vue`
- Test: `web/src/components/notebook/NoteCard.test.ts`, `NoteRow.test.ts`, `CookBadge.test.ts`

**Interfaces:**
- Consumes: `bg-note`, `.mp-grid`, `.mp-display`, `--mp-line`, `--mp-radius-card` (Task 2–3); `initial(name)` z `web/src/today.ts`.
- Produces:
  - `NoteCard.vue` — props `{ label?: string; title?: string; titleTag?: 'h1' | 'h2' | 'h3' }` (domyślnie `titleTag: 'h2'`); sloty `label`, `title` (zastępują propsy), `actions` (prawa strona nagłówka), `subheader` (pasek pod nagłówkiem), `default` (obszar na kratce). DOM: `section.note-card` > `header.note-card__header.bg-note` (z `.note-card__label`, `<component :is="titleTag" class="note-card__title mp-display">`, `.note-card__actions`) > `.note-card__subheader` > `.note-card__body.mp-grid`. Każda część renderuje się tylko, gdy ma treść (prop albo slot). Tytuł z `overflow-wrap: anywhere`.
  - `NoteRow.vue` — props `{ label?: string; highlighted?: boolean; tag?: string }` (domyślnie `highlighted: false`, `tag: 'div'`); sloty `label`, `default`, `action`. DOM: `<component :is="tag" class="note-row" :class="{ 'note-row--highlighted': highlighted }">` > `.note-row__label` > `.note-row__content` > `.note-row__action` (tylko ze slotem). Linia `--mp-line` u dołu, wyróżnienie tłem `surface-variant`; atrybuty (np. `@click`, `aria-current`) przechodzą na korzeń. Treść `min-width: 0` i `overflow-wrap: anywhere`.
  - `CookBadge.vue` — props `{ name: string; color: string; showName?: boolean }` (domyślnie `showName: true`). DOM: `span.cook-badge[title=name]` > `span.cook-badge__avatar` (tło `color`, tekst `initial(name)`, `aria-hidden="true"`) + nazwa, gdy `showName`. Przy `showName: false` nazwa jest tylko w `title` i w ukrytym dla oka `span.d-sr-only`.
  - Wszystkie trzy bez API, store'ów i routera.

- [ ] **Step 1: Napisz testy**

Montowanie przez `mountWith` (Vuetify potrzebny tylko dla klas).

`NoteCard.test.ts`:
- `renders the label and title in the note header`: `{ label: 'Dziś na obiad', title: 'Zupa ogórkowa' }` → `.note-card__header` ma klasę `bg-note`; `.note-card__label` tekst `Dziś na obiad`; `h2.note-card__title` tekst `Zupa ogórkowa`.
- `uses the requested heading level`: `titleTag: 'h1'` → istnieje `h1.note-card__title`, brak `h2`.
- `fills the slots`: slot `title` `<span class="x">Gulasz</span>`, `actions` `<button>+</button>`, `subheader` `Przeliczone`, default `<p>body</p>` → `.note-card__title .x` tekst `Gulasz`; `.note-card__actions button`; `.note-card__subheader` tekst `Przeliczone`; `.note-card__body` ma klasę `mp-grid` i zawiera `p`.
- `omits empty parts`: bez propsów i slotów → brak `.note-card__label`, `.note-card__title`, `.note-card__actions`, `.note-card__subheader`, `.note-card__body`.

`NoteRow.test.ts`:
- `renders the label, content and action`: `{ label: 'Obiad' }`, default `Zupa`, action `<button>v</button>` → `.note-row__label` `Obiad`, `.note-row__content` `Zupa`, `.note-row__action button`.
- `lets the label slot replace the label prop`.
- `shows the highlighted state`: `highlighted: true` → korzeń ma `note-row--highlighted`; `false` → nie ma.
- `renders as the given tag`: `tag: 'li'` → `w.element.tagName === 'LI'`.
- `omits the action column without an action`: brak `.note-row__action`.
- `passes clicks to the root`: `attrs: { onClick: spy }`, klik w `.note-row__content` → `spy` wywołany raz.

`CookBadge.test.ts`:
- `shows the initial and the name`: `{ name: 'Tomek', color: '#123456' }` → `title="Tomek"`, `.cook-badge__avatar` tekst `T`, styl tła `#123456` (`rgb(18, 52, 86)`), tekst zawiera `Tomek`.
- `can hide the visible name`: `showName: false` → widoczny tekst poza `.d-sr-only` to tylko `T`, `title="Tomek"`.

- [ ] **Step 2: Uruchom i sprawdź, że padają**

Run: `cd web && npx vitest run src/components/notebook`
Expected: FAIL (komponenty nie istnieją).

- [ ] **Step 3: Zaimplementuj trzy komponenty**

Wygląd wg `app.css` makiet (karta „Dziś na obiad”: ciemny nagłówek, cień `--shadow-card` z makiety, ramka 1 px `on-surface` w makiecie zastąpiona brakiem ramki zgodnie z §4.2; wiersz: lewa kolumna ok. 100 px z pionową linią `--mp-line`).

- [ ] **Step 4: Uruchom i sprawdź, że przechodzą**

Run: `cd web && npx vitest run src/components/notebook`
Expected: PASS (12 testów).

- [ ] **Step 5: Commit**

```bash
git add web/src/components/notebook
git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit -m "feat(web): note card, ruled row and cook badge components" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Ekran Dziś w nowym układzie

**Files:**
- Modify: `web/src/views/TodayView.vue` (szablon i style; `<script>` bez zmian logiki)
- Modify: `web/src/components/DaySummary.vue`, `web/src/components/QuickAdd.vue` (tylko szablon/style: liniowana sekcja zamiast karty, pole 6 px bez cienia-pigułki; emitowane zdarzenia i propsy bez zmian)
- Test: `web/src/views/TodayView.test.ts`

**Interfaces:**
- Consumes: `NoteCard`, `NoteRow`, `CookBadge` (Task 5); `setViewportWidth` (Task 1); `.mp-num`, `.mp-ruled`, `--mp-rule` (Task 3).
- Produces (kontrakt układu, na którym opierają się testy): bohater jako `NoteCard` (`label` = dotychczasowy `hero-tag`, slot `title` z `span.hero-title`, w treści komórki „`t.cooks` imię” przez `CookBadge`, `portions(n)`, `entryChip`, oraz „Zrobione” (`color="primary"`, wypełniony) i „Przepis” (link, drugorzędny)); stany „wszystko ugotowane” i „nic nie zaplanowane” też jako `NoteCard` z tym samym tytułem co dziś. Posiłki dnia jako `NoteRow` (`label` = `slotTitle(e.slot)`, w treści `button.meal-title`, linia z `CookBadge`, `portions`, chipem kalorii z `title="t.mealNutritionTitle"` i `entryChip`; w slocie `action` przycisk odhaczania z dzisiejszym `aria-label`; klik w wiersz i w `.meal-title` → `openEdit`; odhaczanie i chip z `.stop`). Wolne pory jako `NoteRow` (`label` = pora, przycisk „`t.addMeal`: pora” obrysowany przerywaną linią i „`t.whoCooks`” pod spodem). Pasek dni na obu szerokościach jak dziś (`button[aria-pressed]` z `i` mającym `.off`, gdy dzień bez posiłków) *(decyzja: makieta komputera nie ma paska, ale bez niego zniknęłoby wybieranie dnia)*. Na komputerze `aside` z „`t.thisWeekTitle`” jako 7 `NoteRow` (`t.dayNames[i]`, numer dnia `.mp-num`, tytuły posiłków dnia po przecinku; dziś `highlighted` i `aria-current="date"`), „`t.cooksThisWeek`” jako wiersze z `CookBadge` i liczbą, „`t.openShopping`” jako obrysowany klawisz. Znaczniki przypomnień jako `v-chip` u góry (jak dziś, nowy kształt). Na telefonie bez `aside`, jak dziś.

§4.7 — zachowania, które testy tego zadania trzymają zielone: komunikat błędu z „Spróbuj ponownie”; pasek ładowania; znaczniki przeglądu spiżarni i meal prepu; bohater w trzech stanach; „Zrobione” i „Przepis” z `?servings`; pasek dni z kropką; lista z edycją po kliknięciu, dialog wartości odżywczych z chipu kalorii, avatar gotującego, odhaczanie z `aria-label`; wolne pory otwierające dodawanie; podsumowanie dnia z dopisywaniem i usuwaniem; pusty stan bez przepisów; szybkie dodawanie; panel boczny na komputerze; dialogi `EntryDialog`, `FoodLogDialog`, `MealNutritionDialog`.

Istniejące testy (`lists the day…` na `.meal-title` i `.hero-title`, `marks the hero meal done…`, `opens the recipe reading view…`, przypomnienia) zostają bez zmian — klasy `.meal-title` i `.hero-title` zostają w nowym szablonie.

- [ ] **Step 1: Dopisz testy zachowań (przechodzą na starym układzie)**

`routes()` dostaje opcje `entries`, `recipes`, `members`, `nutrition` (`{ enabled, days }`), `failRecipesOnce`; domyślne wywołania bez zmian. Helpery `clickable`, `openDialog`, `setViewportWidth` z `test/mount.ts` (Task 1). `afterEach` dokłada `setViewportWidth(1024)`.
- `offers a retry when loading fails`: pierwszy `GET /recipes` rzuca → widać `t.retry`; klik → `.hero-title` `Gulasz`.
- `shows a progress bar while loading`: `GET /recipes` → `new Promise(() => {})` → istnieje `.v-progress-linear`.
- `shows the finished and the empty hero`: wszystkie wpisy `done: true` → tekst zawiera `t.allCooked`; brak wpisów → `t.nothingPlanned` i `t.nothingPlannedHint`.
- `switches the day with the strip`: 7 `button[aria-pressed]`, trzeci (`2026-11-11`) `aria-pressed="true"` i zawiera `i:not(.off)`, czwarty zawiera `i.off`; klik czwartego → on ma `aria-pressed="true"`, tekst zawiera `t.nothingPlanned`.
- `opens a meal for editing`: klik pierwszego `.meal-title` → dialog zawiera `t.editEntryTitle`.
- `toggles a meal without opening it` (Review Focus): klik `button[aria-label="${t.markDone}: Gulasz"]` (pierwszy) → `PUT /plan/2026-11-09/entries/e1` z `done: true`; brak `.v-overlay--active`.
- `opens the meal nutrition from the calorie chip`: `nutrition.enabled`, dzień `2026-11-11` z `meals: [{ entryId: 'e1', values: { kcal: 500, … } … }]` → klik `[title="${t.mealNutritionTitle}"]` → dialog otwarty i nie zawiera `t.editEntryTitle`.
- `shows who cooks`: `e1` z `cookUserId: 'u1'` → istnieje `[title="Marc"]` z tekstem zawierającym `M`.
- `skips an unknown cook` (Review Focus): `cookUserId: 'gone'` → tekst widoku nie zawiera `undefined`, liczba `[title="Marc"]` = 0.
- `adds a meal in a free slot`: klik przycisku z tekstem zawierającym `${t.addMeal}: ${t.slotNames.snack.toLowerCase()}` → dialog zawiera `t.addEntryTitle`.
- `logs food in the day summary`: dzień z `log: [{ id: 'l1', name: 'Jabłko', … }]` → tekst zawiera `t.daySummary`; `buttonByText(t.logAdd)` → dialog otwarty; klik `button[title="${t.logRemove}"]` → `DELETE /me/food-log/l1` w `calls`.
- `shows the empty state without recipes`: `recipes: []`, `entries: []` → tekst `t.noRecipes`, link z `href` zawierającym `/recipe-new`.
- `offers quick add`: istnieje `input[placeholder="${t.quickPlaceholder}"]`.
- `shows the side panel only on desktop`: 1280 → tekst zawiera `t.cooksThisWeek` i link `t.openShopping`; 390 → nie zawiera żadnego.

Run: `cd web && npx vitest run src/views/TodayView.test.ts`
Expected: PASS na starym układzie. Commit:

```bash
git add web/src/views/TodayView.test.ts
git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit -m "test(web): pin today view behaviour before the redesign" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 2: Dopisz testy nowego układu**

- `shows the next meal in a note card`: `.note-card .note-card__label` tekst `${t.todayOn} ${t.slotOn.lunch}`; `.note-card .hero-title` `Gulasz`; `.note-card` zawiera `portions(4)`, przycisk `t.markDone` i link `t.openRecipe`.
- `lists the day as ruled rows with the meal time on the left`: `.note-row` zawierające `.meal-title` mają `.note-row__label` = `[t.slotNames.breakfast, t.slotNames.lunch, t.slotNames.dinner]`; istnieje `.note-row` z labelem `t.slotNames.snack` i przyciskiem `t.addMeal`; `.meal-title` to `BUTTON` (Review Focus).
- `shows this week as rows with today highlighted` (1280): `aside .note-row` — 7; `aside .note-row--highlighted` — 1, z `aria-current="date"`, zawiera `t.dayNames[2]`, `11` i `Gulasz`.
- `has no side panel on a phone` (390): brak `aside`; 7 `button[aria-pressed]`.

- [ ] **Step 3: Uruchom i sprawdź, że nowe padają**

Run: `cd web && npx vitest run src/views/TodayView.test.ts`
Expected: FAIL tylko 4 testy ze Step 2.

- [ ] **Step 4: Przebuduj szablon `TodayView.vue`, `DaySummary.vue`, `QuickAdd.vue`**

Układ wg `screenshots/dzis-desktop.png` i `dzis-phone.png`; kontrakt w Interfaces. Usuń style `.hero*`, `.tile`, `.free`, `.mini`, które przestały być używane. „Zrobione” w wierszu (odhaczony) w kolorze `success`. Kalorie i liczby z `.mp-num`. `.quick-dock` dalej przyklejony nad dolnym paskiem (72 px).

- [ ] **Step 5: Uruchom i sprawdź, że wszystko przechodzi**

Run: `cd web && npx vitest run src/views/TodayView.test.ts src/components/EntryDialog.test.ts`
Expected: PASS (wszystkie stare, przypinające i nowe).

- [ ] **Step 6: Commit**

```bash
git add web/src/views/TodayView.vue web/src/views/TodayView.test.ts web/src/components/DaySummary.vue web/src/components/QuickAdd.vue
git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit -m "feat(web): today screen as a notebook page" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Przepis w nowym układzie

**Files:**
- Modify: `web/src/views/RecipeView.vue` (szablon i style; `<script>` dostaje tylko `const { mdAndUp } = useDisplay()`)
- Test: `web/src/views/RecipeView.test.ts`

**Interfaces:**
- Consumes: `NoteCard` (Task 5); `setViewportWidth` (Task 1); `.mp-num`, `--mp-rule` (Task 3).
- Produces (kontrakt układu): korzeń `div.recipe-view` z klasą `recipe-view--columns` przy `mdAndUp`; nagłówek: link powrotu (`t.recipes`, tekstowy), `h1` z tytułem, tagi, po prawej link źródła i „Edytuj” (obrysowany, ikona ołówka) — oba jak dziś (`href`, `target`, `rel`); pod nagłówkiem linia `.nutrition` z tym samym tekstem co dziś. Dalej `.recipe-view__aside` z `NoteCard` (`title` = `t.ingredients` tylko gdy są składniki; slot `actions` z dzisiejszą grupą `[role="group"][aria-labelledby="servings-label"]`, `#servings-label` = `t.servingsLabel`, przyciski z `aria-label` `t.fewerServings`/`t.moreServings` i hit targetem ≥ 44 px, liczba z `aria-live`; slot `subheader` z „`t.scaledFrom(base)` · `.restore`” tylko przy przeliczeniu; w treści `ul` z `li.ingredient` tylko gdy są składniki) i `.recipe-view__main` z `h2` `t.steps` i `ol.steps` dużą czcionką. Na komputerze aside po lewej, `position: sticky`; na telefonie jedna kolumna. *(decyzja: bez składników karta zostaje — bez tytułu i bez listy — bo trzyma licznik porcji; makro w osobnych kolumnach z makiety nie wchodzą, bo zmieniłyby tekst linii `.nutrition`, który przypinają testy)*
- `li.ingredient`: ilość w osobnym elemencie (prawa kolumna `.mp-num`), nazwa obok, notatka pod nazwą w `text-medium-emphasis`; między ilością a nazwą zostaje dosłowna spacja w DOM, bo `li.text()` musi dalej dawać `300 g ziemniak`, a składnik bez ilości zaczynać się od `sól i pieprz`.

§4.7 — wszystko z `2026-11-17-recipe-view-design.md` §4.2–§4.3 (nagłówek, wartości na porcję, licznik 1–100 z `router.replace`, „Przeliczone z… · Przywróć”, składniki w jednej linii, kroki numerowane, źródło, 404 i błąd). Pilnują tego istniejące testy `RecipeView.test.ts` — zostają bez zmian (nie używają znaczników Vuetify, których nie będzie).

- [ ] **Step 1: Dopisz testy nowego układu**

`afterEach` dokłada `setViewportWidth(1024)`.
- `puts the ingredients and the servings stepper in a note card`: `.note-card .note-card__header [role="group"]` istnieje; `.note-card__title` tekst `t.ingredients`; `.note-card li.ingredient` — 4.
- `shows the scaling note under the card header`: `?servings=4` → `.note-card__subheader` zawiera `t.scaledFrom(2)` i `.restore`; bez `?servings` brak `.note-card__subheader`.
- `keeps the stepper when there are no ingredients`: `ingredients: []` → `[role="group"]` istnieje, brak `.note-card__title`, tekst nie zawiera `t.ingredients`.
- `uses two columns on desktop and one on a phone`: 1280 → `.recipe-view--columns` istnieje, `.recipe-view__aside .note-card` i `.recipe-view__main ol.steps` istnieją; 390 → brak `.recipe-view--columns`, `ol.steps > li` — 2.

- [ ] **Step 2: Uruchom i sprawdź, że nowe padają**

Run: `cd web && npx vitest run src/views/RecipeView.test.ts`
Expected: FAIL tylko 4 nowe testy.

- [ ] **Step 3: Przebuduj szablon `RecipeView.vue`**

Wg `screenshots/przepis-desktop.png` i `przepis-phone.png`; szerokość jak treść aplikacji (`--mp-content-max`) zamiast 720 px; na komputerze kolumny ok. 450 px / reszta.

- [ ] **Step 4: Uruchom i sprawdź, że wszystko przechodzi**

Run: `cd web && npx vitest run src/views/RecipeView.test.ts`
Expected: PASS (stare bez zmian + 4 nowe).

- [ ] **Step 5: Commit**

```bash
git add web/src/views/RecipeView.vue web/src/views/RecipeView.test.ts
git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit -m "feat(web): recipe view with ingredients in a note card" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Plan w nowym układzie

**Files:**
- Modify: `web/src/views/PlanView.vue` (szablon i style; `<script>` bez zmian logiki, dochodzi tylko `avatarColor`/`CookBadge` dla osoby)
- Modify: `web/src/locales/pl.ts`, `web/src/locales/en.ts` (nagłówek kolumny dni)
- Test: `web/src/views/PlanView.test.ts`

**Interfaces:**
- Consumes: `NoteRow`, `CookBadge` (Task 5); `avatarColor(id, ids)` i `portions(n)`; kontrakt testów z Task 1.
- Nowy klucz: `planDayColumn: 'Dzień'` (en: `'Day'`).
- Produces (kontrakt układu):
  - nagłówek: `button[title=t.prevWeek]` (i `aria-label`), `h1` z zakresem, `button[title=t.nextWeek]`, „`t.thisWeek`” poza bieżącym tygodniem, po prawej „`t.copyPrevWeek`” obrysowany (`:disabled="!editable"`). Alert błędu z „`t.retry`” i komunikat sukcesu jak dziś.
  - komputer: `<table class="plan-table">` — `thead` z `th` `t.planDayColumn` + `SLOTS` (tytuły); 7 wierszy `tbody tr`, pierwsza komórka `th scope="row"` z dniem (`formatDay`), `.today-tag` przy dziś i linią kalorii (`lineOf(d)` + ikona ostrzeżenia z `aria-label`/`title` = `warnOf(d)`); dzisiejszy wiersz ma klasę `plan-row--today` (tło `surface-variant`) i `aria-current="date"`; komórka: każdy wpis jako `button.plan-entry` (tytuł podkreślony, linia `CookBadge` + `portions(e.servings)`, dopisek `entryChip`), pusta i niepusta komórka ma przycisk „+” z dzisiejszym `aria-label`; w pustej komórce przycisk ma też widoczny tekst `t.addEntryTitle` (jak w makiecie). Tabela w kontenerze z `overflow-x: auto` tylko jako zabezpieczenie.
  - telefon: 7 bloków `section.plan-day` (dzisiejszy z `plan-day--today` i `aria-current="date"`): nagłówek z dniem, `.today-tag` i linią kalorii (z widocznym tekstem ostrzeżenia, jak dziś); wpisy jako `NoteRow` (`label` = tytuł pory, w treści `button.plan-entry` jak wyżej); pod spodem przycisk tekstowy „`t.addEntryTitle`” z `aria-label` `${t.addEntryTitle}: ${formatDay(d)}`, otwierający dodawanie z porą `dinner` jak dziś.

§4.7 — testy z Task 1 muszą przejść bez zmian asercji: przełączanie tygodni, „Ten tydzień”, kopiowanie i jego blokada, błąd z ponowieniem, komunikat sukcesu, pasek ładowania, linia kalorii z ostrzeżeniem, wyróżnienie dziś, dodawanie z opisem, edycja po kliknięciu, dopisek wpisu, `EntryDialog`.

- [ ] **Step 1: Dopisz testy nowego układu**

- `shows a table with a row per day on desktop` (1280): `thead th` teksty = `[t.planDayColumn, ...SLOTS.map((s) => s.title)]`; `tbody tr` — 7; `tr.plan-row--today` — 1, `aria-current="date"`, zawiera `formatDay('2026-11-11')`; `button.plan-entry` z `Gulasz` zawiera `portions(4)` i `Marc`.
- `shows a block per day with ruled rows on a phone` (390): `section.plan-day` — 7; `.plan-day--today` — 1; w bloku `2026-11-09` `.note-row__label` = `t.slotNames.lunch`; przyciski dodawania mają `aria-label` `${t.addEntryTitle}: ${formatDay(d)}` dla każdego dnia.
- (oba) `makes entries keyboard buttons`: element z `Gulasz` z `clickable` to `BUTTON` (Review Focus).
- (oba) `shows two entries of the same meal time` (Review Focus): dodatkowy `p4` 09.11 obiad `freeText: 'Sałatka'` → widać `Gulasz` i `Sałatka`; `clickable('Sałatka').click()` → dialog z `t.editEntryTitle`.
- (oba) `skips an unknown cook` (Review Focus): `p1.cookUserId = 'gone'` → tekst nie zawiera `undefined`, brak `.cook-badge`.

- [ ] **Step 2: Uruchom i sprawdź, że nowe padają**

Run: `cd web && npx vitest run src/views/PlanView.test.ts`
Expected: FAIL tylko nowe testy; testy z Task 1 PASS.

- [ ] **Step 3: Dodaj klucz tłumaczeń i przebuduj szablon `PlanView.vue`**

Wg `screenshots/plan-desktop.png` i `plan-phone.png`, kontrakt w Interfaces. Usuń style `.plan-grid`, `.today-card`.

- [ ] **Step 4: Uruchom i sprawdź, że wszystko przechodzi**

Run: `cd web && npx vitest run src/views/PlanView.test.ts src/i18n.test.ts src/messages.test.ts`
Expected: PASS (testy z Task 1 bez zmian + nowe).

- [ ] **Step 5: Commit**

```bash
git add web/src/views/PlanView.vue web/src/views/PlanView.test.ts web/src/locales/pl.ts web/src/locales/en.ts
git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit -m "feat(web): plan as one line per day, a block per day on phones" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Dokumentacja, pełna weryfikacja i kontrola na zrzutach

**Files:**
- Modify: `CHANGELOG.md`, `docs/roadmap.md`, `docs/design/2026-11-redesign/README.md`
- Create (poza repo, w katalogu tymczasowym sesji): skrypt `shots.cjs`, baza `shots.db`, katalog `shots/` — nie commitujemy

- [ ] **Step 1: Dokumentacja**

`CHANGELOG.md`, w „Unreleased” nowa sekcja `### Nowy wygląd` (pierwsza): czcionki Red Hat dołączone do aplikacji (bez zewnętrznych serwerów); prostokątne przyciski, karty bez ramek, liniowane wiersze; na komputerze górny pasek z linkami (w tym Meal prep) zamiast bocznego, nazwa gospodarstwa w menu konta na węższych ekranach; ekran Dziś z kartą „co teraz gotujemy” i listą pór; przepis ze składnikami w karcie z licznikiem porcji i krokami obok; plan z wierszem na dzień (na telefonie blok na dzień); 5 palet i tryb ciemny zostają, nowy kolor nagłówka karty w każdej palecie.
`docs/roadmap.md`: w tabeli punkt 6 z dopiskiem „— **zrobione**”, nagłówek `## 6. Nowy wygląd (zrobione)`, „Do decyzji” z odpowiedziami: wersja ciemna i cztery zapasowe palety zostają; zostajemy na Vuetify 3 (Vuetify 4 to dalej punkt 9).
`docs/design/2026-11-redesign/README.md`: w pierwszym akapicie zdanie, że wygląd jest wdrożony w aplikacji (listopad 2026), a makiety zostają jako wzorzec.

- [ ] **Step 2: Pełna weryfikacja**

Run: `cd web && npx vitest run && npm run build; git restore dist/.gitkeep`
Expected: wszystkie testy PASS (pojedyncze przekroczenia czasu — uruchom te pliki osobno, mają przejść), `vue-tsc` bez błędów, build OK, `web/dist/.gitkeep` istnieje.
Run (z katalogu repo): `go test ./docs/`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md docs/roadmap.md docs/design/2026-11-redesign/README.md
git -c user.name=MarcinCho -c user.email=MarcinCho@users.noreply.github.com commit -m "docs: notebook redesign shipped" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 4: Zrzuty (ręczna kontrola §4.8)**

Uruchomienie (`$TMP` = katalog tymczasowy sesji; oba procesy w tle):
- backend z katalogu repo: `MEALPLAN_ADDR=127.0.0.1:8080 MEALPLAN_COOKIE_SECURE=false MEALPLAN_DB_DSN="file:$TMP/shots.db?_pragma=foreign_keys(1)" go run ./cmd/mealplan`; gotowe, gdy `curl -s localhost:8080/api/v1/auth/status` zwraca `setupRequired: true`;
- frontend: `cd web && npm run dev -- --port 5173 --strictPort` (Vite przekazuje `/api` na 8080).

Skrypt `$TMP/shots.cjs` z `require('C:/TMP/Meal_plan/redesign/node_modules/puppeteer-core')`, `executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe'`, na `http://localhost:5173`:
1. Dane przez `page.evaluate(fetch)` na tym samym originie (ciastka sesji zostają; `X-CSRF-Token` z ciastka `mealplan_csrf`): `POST /api/v1/auth/setup` z testowym administratorem (pola wg `handleSetup` w `internal/httpapi`; hasło losowe generowane w skrypcie, nigdzie nie wypisywane), nazwa gospodarstwa (z setupu albo `PUT /api/v1/household/settings`), przepis „Zupa ogórkowa” (2 porcje, 6 składników, w tym z notatką i bez ilości, 4 kroki, źródło), przepis z 80-znakowym tytułem bez spacji (Review Focus), `PUT /api/v1/me/nutrition` z `enabled: true` i celem 2200 kcal, wpisy bieżącego tygodnia: dziś śniadanie (zrobione), dziś obiad z kucharzem, inny dzień z długim tytułem, wyjście na miasto.
2. Dla motywów `{mealplan, light}`, `{mealplan, dark}`, `{classic, light}` (najbardziej zmieniona paleta: biały pas i nowe `surface-variant`) ustaw `localStorage['mealplan.theme']` = `JSON.stringify({ palette, mode })` i przeładuj.
3. Dla rozmiarów 390 × 844 i 1440 × 900 i tras `/`, `/plan`, `/recipes/<id zupy>?servings=4`: `goto` z `waitUntil: 'networkidle0'`, zrzut całej strony do `$TMP/shots/<ekran>-<paleta>-<tryb>-<szerokość>.png` (18 plików) i zapis `document.documentElement.scrollWidth > innerWidth`.

Expected: skrypt wypisuje `overflow: none` dla wszystkich 9 zrzutów 390 px. Obejrzyj każdy zrzut (Read) obok `docs/design/2026-11-redesign/screenshots/` i sprawdź: nagłówek karty odcina się od karty i ma czytelny tekst; kratka widoczna, ale spokojna; „Zrobione” w kolorze `success`; dolny pasek mieści 6 zakładek; górny pasek przy 1440 ma 6 linków i nazwę gospodarstwa; brak czcionki zastępczej (Red Hat widoczny); żaden tekst nie wychodzi poza ekran. Usterki poprawiasz w zadaniu, którego dotyczą (osobny commit `fix(web): …`), i powtarzasz ten krok. Na koniec zatrzymaj oba procesy i usuń `$TMP/shots.db*`. Zrzutów nie commitujemy.
