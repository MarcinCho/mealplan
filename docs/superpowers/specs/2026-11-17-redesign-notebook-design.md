# Mealplan: nowy wygląd „notes” w aplikacji

Data: 2026-11-17 · Status: zatwierdzony i wdrożony (2026-11-17) · Punkt 6 z `docs/roadmap.md` · Wzorzec: `docs/design/2026-11-redesign/`

## 1. Po co

Właściciel wybrał kierunek wyglądu „spokojnie jak notes” (makiety w `docs/design/2026-11-redesign/`) i chce, żeby stał się głównym wyglądem aplikacji. Dziś aplikacja ma wygląd Material 3 z Vuetify: przyciski-pigułki, karty z ramką, duże zaokrąglenia, czcionka systemowa, boczny pasek nawigacji na komputerze. Kolory marki już są (paleta „mealplan” w `web/src/theme.ts`), ale charakteru notesu nie ma.

**Kryteria sukcesu**
- Ekrany Dziś, Przepis i Plan wyglądają jak makiety (układ, czcionki, karta z notesu, liniowane wiersze), na telefonie i komputerze.
- Wszystkie pozostałe ekrany od razu dostają nowe czcionki, kształty przycisków i pól oraz nawigację, bez przebudowy układu.
- Każda z 5 palet w trybie jasnym i ciemnym działa w nowym wyglądzie i spełnia WCAG AA (test).
- Nic z dzisiejszych funkcji tych ekranów nie znika.

## 2. Zakres

1. **System wyglądu dla całej aplikacji:** czcionki Red Hat, kształty i domyślne warianty komponentów Vuetify, liniowane powierzchnie zamiast kart z ramką.
2. **Nawigacja:** górny pasek z logo i linkami na komputerze (zamiast bocznego paska), dolny pasek zakładek na telefonie; wszystkie 6 miejsc, także „Meal prep”.
3. **Komponenty „notesu”:** karta z notesu i liniowany wiersz, używane na trzech ekranach.
4. **Przebudowa układu:** Dziś, Przepis (`RecipeView`), Plan.
5. **Palety:** 5 palet × jasny/ciemny w nowym wyglądzie, test kontrastu dla nowych par kolorów.

**Poza zakresem (non-goals)**
- Przebudowa układu pozostałych ekranów (Spiżarnia, Zakupy, Meal prep, Przepisy-lista, edytor przepisu, Produkty, Domownicy, Ustawienia, Konto, logowanie, import, „Sprawdź…”). Dostają tylko system wyglądu z punktu 1 i nawigację.
- Vuetify 4 (osobny punkt 9 roadmapy).
- Nowe funkcje, zmiany w API, danych i logice. Zmiany dotyczą szablonów i stylów; logika komponentów zostaje.
- Zmiana tekstów, poza miejscami, gdzie nowy układ rozdziela dziś sklejony tekst (np. „Obiad · 4 porcje” staje się kolumną pory posiłku i osobną linią).
- Stany ładowania i błędu inne niż te, które ekrany już mają.

**Anty-cele (porażka, nawet jeśli działa)**
- Któraś akcja z dzisiejszych ekranów znika albo jest trudniej dostępna na telefonie (np. odhaczanie posiłku, szybkie dodawanie, dialog wartości odżywczych, edycja wpisu po kliknięciu, kopiowanie tygodnia).
- Którakolwiek paleta lub tryb ciemny wygląda na zepsuty (nieczytelny tekst, kratka niewidoczna albo krzykliwa, „Zrobione” w złym kolorze).
- Aplikacja pobiera czcionki z zewnętrznego serwera.
- Testy widoków przestają sprawdzać zachowanie, bo zostały przepisane pod nowy HTML zamiast poprawione.

## 3. Podejścia

| | Podejście | Za | Przeciw |
|---|---|---|---|
| A | Sam Vuetify: konfiguracja (czcionki, kształty, warianty) i trzy ekrany z gotowych komponentów | najmniej kodu, jeden system | karta z kratką i liniowane wiersze trudne do wiernego złożenia; wychodzi „Vuetify w nowych kolorach” |
| **B (wybrane)** | Konfiguracja Vuetify dla całej aplikacji + 2–3 własne komponenty prezentacyjne (karta z notesu, liniowany wiersz) na kolorach palety | wierne makietom tam, gdzie widać; przyciski, pola, menu i dostępność z Vuetify; reszta ekranów zmienia się sama | trochę własnego CSS |
| C | Przeniesienie HTML/CSS makiet 1:1 bez Vuetify na trzech ekranach | najwierniejsze | dwa systemy stylów, ręczne podpinanie palet, stany i dostępność od nowa |

Właściciel wybrał B. Wcześniej wybrał też, że **5 palet i tryb ciemny zostają** (zamiast jednego wyglądu w kolorach marki).

## 4. Projekt

### 4.1 Czcionki

- Red Hat Display (nagłówki, tytuły posiłków i przepisów, duże liczby) i Red Hat Text (cała reszta). Ilości składników i liczby w tabelach mają cyfry o stałej szerokości (`font-variant-numeric: tabular-nums`). *(decyzja projektanta: makiety używały też Red Hat Mono do pojedynczych liczb; dwie rodziny wystarczą)*
- Pliki czcionek z paczek npm (`@fontsource/red-hat-display`, `@fontsource/red-hat-text`, tylko używane grubości i podzbiory latin oraz latin-ext dla polskich znaków), wbudowane przez Vite. Bez Google Fonts. *(decyzja projektanta: selfhost i prywatność)*
- Vuetify dostaje te rodziny jako czcionkę treści i nagłówków (klasy typografii `text-h*` też).

### 4.2 Kształty i domyślne warianty (`web/src/plugins/vuetify.ts`)

Zamiast obecnych „Material 3” (pigułki, `rounded: 'xl'`, ramki):
- **Przyciski:** prostokątne klawisze z zaokrągleniem 6 px, bez wersalików, główna akcja wypełniona kolorem `primary`, drugorzędne obrysowane. Przyciski-ikony okrągłe jak dziś.
- **Karty:** zaokrąglenie 10 px, bez ramki i cienia domyślnie; tło `surface`.
- **Pola formularzy:** obrysowane, zaokrąglenie 6 px.
- **Chipy i alerty:** zaokrąglenie 6 px.
- **Listy:** bez zaokrągleń; wiersze rozdzielone cienką linią tam, gdzie lista stoi na tle strony.
- **Menu, pasek komunikatów (snackbar), dialogi:** zaokrąglenie 10 px (menu konta dziś ma `rounded="xl"`, pasek komunikatów `rounded="pill"`; oba przechodzą na nowe wartości).
Dokładne wartości (wysokości, grubości) zostają wykonawcy, w zgodzie z makietami.

**Zasięg zmian w przyciskach:** domyślne ustawienia i kształt (zaokrąglenie 6 px, bez wersalików) dotyczą wszystkich przycisków w aplikacji. Wariantów ustawionych jawnie w szablonach (`variant="tonal"`, `"text"` itp.) poza trzema przebudowanymi ekranami i nawigacją **nie zmieniamy**; tonalne przyciski na innych ekranach zostają tonalne, tylko w nowym kształcie. Na trzech ekranach warianty dobiera się według makiet (główna akcja wypełniona, reszta obrysowana lub tekstowa). *(decyzja projektanta: przegląd wszystkich szablonów to osobna praca, a kształt i czcionki już dają spójny wygląd)*

### 4.3 Kolory i palety (`web/src/theme.ts`)

Elementy notesu używają kolorów palety; prawie wszystkie już istnieją, dochodzi jedna para (`note` / `on-note`):

| Element | Kolor palety |
|---|---|
| tło strony | `background` |
| karty, wiersze | `surface` |
| kratka w karcie z notesu, wyróżniony wiersz („Dziś”) | `surface-variant` |
| nagłówek karty z notesu i jego tekst | **nowe** `note` / `on-note` |
| główna akcja („Zrobione”), zaznaczenie, znacznik „Dziś” | `primary` / `on-primary` |
| odhaczony posiłek | `success` |
| tekst i linie | `on-surface` (linie jako przezroczysta odmiana, np. `color-mix` lub `rgba(var(--v-theme-on-surface), …)`) |

- **Nowa para kolorów `note` / `on-note`** w każdej palecie i trybie (10 par), bo `bar` nie nadaje się wszędzie: w „classic” jasnej pasek jest biały, a w trybach ciemnych `bar` bywa równy `surface`. Zasada: w trybie jasnym `note` to ciemny pas (jak `#233831` w „mealplan”; dla „classic” ciemny niebieskoszary, np. `#37474F`); w trybie ciemnym jasny pas na ciemnej karcie (odwrócenie, np. jasna zieleń szałwiowa dla „mealplan”). Dokładne wartości dobiera wykonawca. *(decyzja projektanta: nagłówek karty to wyróżnik notesu, więc musi odcinać się od karty w każdej palecie)*
- **„classic” dostaje jawne `surface-variant`** w obu trybach (dziś go nie ma i Vuetify podstawia własny), żeby kratka i wyróżniony wiersz były widoczne. W pozostałych paletach wykonawca poprawia `surface-variant` tylko wtedy, gdy kratka jest niewidoczna. *(decyzja projektanta)*
- Test kontrastu w `web/src/theme.test.ts` obejmuje każdą z 10 kombinacji: `on-surface` na `surface` i `background` (4,5:1), `on-surface` na `surface-variant` (4,5:1), `on-note` na `note` (4,5:1), `note` na `surface` (3:1, żeby pas odcinał się od karty), `on-primary` na `primary` (4,5:1), `success` na `surface` jako element interfejsu (3:1), oraz że każda paleta definiuje `surface-variant`, `note` i `on-note`.
- Przełącznik palety i trybu zostaje tam, gdzie jest (menu konta w `AppShell.vue`).

### 4.4 Nawigacja (`web/src/layouts/AppShell.vue`)

- **Komputer (od `md`):** górny pasek na tle strony z cienką linią u dołu: logo, linki tekstowe Dziś, Plan, Meal prep, Przepisy, Spiżarnia, Zakupy, po prawej nazwa gospodarstwa (jak dziś) i menu konta. Gdy szerokość nie mieści nazwy (np. przy 960 px), nazwa chowa się z paska i pojawia się na górze menu konta; nigdy nie znika całkiem. *(decyzja projektanta)* Aktywny link wyróżniony tłem `surface-variant` albo podkreśleniem w kolorze `primary`. Boczny pasek (`v-navigation-drawer` „rail”) znika.
- **Telefon:** dolny pasek zakładek z ikoną i podpisem dla tych samych 6 miejsc, jak dziś, w nowym wyglądzie; logo w górnym pasku jako sam znak.
- Treść ma ograniczoną szerokość i jest wyśrodkowana na szerokich ekranach (jak makiety: ok. 1150 px treści).
- Kolejność linków jak dzisiaj (`items` w `AppShell.vue`).

### 4.5 Komponenty notesu (`web/src/components/notebook/`)

- **`NoteCard`:** karta z notesu. Nagłówek w kolorze `note` z małym podpisem (np. „Dziś na obiad”) i dużym tytułem (sloty), pod nim obszar na kratce (`surface-variant` z rysowaną kratką) na komórki i akcje (domyślny slot). Używana na Dziś (bieżący posiłek) i w Przepisie (składniki z licznikiem porcji w nagłówku).
- **`NoteRow`:** liniowany wiersz z lewą kolumną etykiety (np. pora posiłku, dzień), treścią i miejscem na akcję po prawej (np. odhaczenie). Cienka linia pod wierszem. Używany na Dziś (posiłki dnia, wolne pory), w Planie (dzień na telefonie) i w panelu „Ten tydzień”.
- Komponenty są czysto prezentacyjne (sloty, propsy na teksty i stan `highlighted`), bez wywołań API i bez logiki widoków.
Wykonawca może dodać trzeci mały komponent, jeśli ten sam fragment powtarza się w dwóch ekranach (np. komórka „Gotuje Tomek” / „4 porcje”).

### 4.6 Ekrany

Układ według zrzutów w `docs/design/2026-11-redesign/screenshots/` i makiet HTML obok. Każdy ekran zachowuje wszystkie dzisiejsze elementy i akcje (lista w §4.7).

- **Dziś (`TodayView.vue`):** powitanie i data; bieżący posiłek jako `NoteCard` („Dziś na obiad”, tytuł, komórki „Gotuje …” i „N porcji”, przyciski „Zrobione” i „Przepis”); pod nią posiłki dnia jako `NoteRow` z porą w lewej kolumnie, osobą, porcjami, kaloriami i odhaczaniem; wolne pory jako `NoteRow` z „Dodaj: …” i „Kto ugotuje?”; szybkie dodawanie; podsumowanie dnia. Na komputerze prawa kolumna: „Ten tydzień” (dzień, data, posiłki dnia jako lista wierszy; dziś wyróżnione), „Kto gotuje w tym tygodniu”, „Otwórz zakupy”. Na telefonie pasek dni nad listą posiłków, jak dziś. Przypomnienia (przegląd spiżarni, meal prep) jako przyciski-znaczniki u góry.
- **Przepis (`RecipeView.vue`):** nagłówek z powrotem do listy, tytułem, tagami i „Edytuj”; wartości odżywcze na porcję; składniki w `NoteCard`, w której nagłówku jest licznik porcji (− / liczba / +) i pod nim pasek „Przeliczone z N porcji · Przywróć”; kroki jako ponumerowana lista dużą czcionką; link do źródła. Na komputerze dwie kolumny: karta składników po lewej, przyklejona przy przewijaniu; kroki po prawej. Na telefonie jedna kolumna. Zachowanie (adres `?servings`, zaokrąglanie, dostępność licznika) bez zmian.
- **Plan (`PlanView.vue`):** nagłówek tygodnia z przyciskami poprzedni/następny, „Ten tydzień” i „Skopiuj z poprzedniego tygodnia”. Na komputerze tabela: wiersz na dzień, kolumny na pory posiłków, dzisiejszy wiersz wyróżniony i oznaczony „Dziś”, linia kalorii pod nazwą dnia; wpis to klikalny tekst (tytuł, osoba, porcje, dopisek meal prep), pusta komórka ma „+”. Na telefonie każdy dzień jako blok z nagłówkiem dnia i wierszami `NoteRow` na wpisy, przycisk „Dodaj posiłek” (jak dziś otwiera dodawanie z porą „Kolacja”, którą można zmienić w dialogu). *(decyzja projektanta: makiety mają dni w wierszach; dziś aplikacja ma dni w kolumnach; zachowanie „Dodaj posiłek” bez zmian)*

**Próg telefon/komputer:** wszędzie `mdAndUp` z Vuetify (960 px): nawigacja, panel boczny Dziś, dwie kolumny i przyklejona karta w Przepisie, tabela w Planie. *(decyzja projektanta: makiety przełączają się przy 60rem, ale aplikacja konsekwentnie używa `mdAndUp`)*

### 4.7 Co musi zostać na trzech ekranach (z kodu na `master`)

- **Dziś:** komunikat błędu z „Spróbuj ponownie”; pasek ładowania; znaczniki „Czas na przegląd spiżarni” i przypomnienie meal prep; bohater w trzech stanach (następny posiłek, wszystko ugotowane, nic nie zaplanowane); „Zrobione” i „Przepis” (z `?servings`); pasek dni z kropką dla dni z posiłkami; lista posiłków dnia z kliknięciem otwierającym edycję, znacznikiem kalorii otwierającym dialog wartości odżywczych, avatarem gotującego i odhaczaniem (z `aria-label`); wolne pory otwierające dodawanie; podsumowanie dnia z dopisywaniem i usuwaniem wpisów dziennika; pusty stan bez przepisów; szybkie dodawanie; panel boczny na komputerze; dialogi `EntryDialog`, `FoodLogDialog`, `MealNutritionDialog`.
- **Przepis:** wszystko z `docs/superpowers/specs/2026-11-17-recipe-view-design.md` §4.2–§4.3, w tym stany 404 i błędu.
- **Plan:** przełączanie tygodni, „Ten tydzień” poza bieżącym tygodniem, „Skopiuj z poprzedniego tygodnia” (wyłączone, gdy nie wolno), komunikat błędu z ponowieniem, komunikat sukcesu, pasek ładowania, linia kalorii dnia z ostrzeżeniem, wyróżnienie dzisiejszego dnia, dodawanie w każdej komórce/dniu z opisem dla czytników ekranu, edycja wpisu po kliknięciu, dopisek wpisu (meal prep, wyjście), dialog `EntryDialog`.

### 4.8 Testy

- **Najpierw testy Planu:** `PlanView.vue` nie ma dziś testów, więc przed przebudową powstaje `PlanView.test.ts`, który na starym układzie sprawdza zachowania z §4.7 (przełączanie tygodni, „Ten tydzień”, kopiowanie tygodnia i jego blokada, wyróżnienie dzisiejszego dnia, dodawanie z opisem dla czytników ekranu, edycja po kliknięciu, dopisek wpisu, błąd z ponowieniem) na telefonie i komputerze; po przebudowie ten sam test przechodzi. *(decyzja projektanta)*
- Istniejące testy widoków (`TodayView.test.ts`, `RecipeView.test.ts`, `AppShell.test.ts`, testy innych widoków) przechodzą. Zmiany w nich tylko tam, gdzie test opiera się na znacznikach Vuetify, których już nie ma (np. `v-navigation-drawer`), i zawsze z zachowaniem tego, co test sprawdza.
- Test motywu: kontrast z §4.3 dla wszystkich 10 kombinacji; czcionka treści to Red Hat Text.
- Test `AppShell`: 6 linków nawigacji na komputerze i na telefonie, aktywny link oznaczony (`aria-current="page"`).
- Testy `NoteCard` i `NoteRow`: wyświetlają sloty i stan wyróżnienia.
- Ręczna kontrola na zrzutach (telefon 390 px i komputer 1440 px) dla trzech ekranów w palecie „mealplan” jasnej i ciemnej oraz jednej innej palecie; bez poziomego przewijania przy 390 px.

### 4.9 Dokumentacja

- `CHANGELOG.md`: sekcja „Nowy wygląd” w „Unreleased”.
- `docs/roadmap.md`: punkt 6 oznaczony jako zrobiony, z odpowiedziami na „Do decyzji” (palety i tryb ciemny zostają; Vuetify 3).
- `docs/design/2026-11-redesign/README.md`: zdanie, że wygląd jest wdrożony w aplikacji.

## 5. Zostawione wykonawcy

- Dokładne wartości odstępów, wysokości przycisków, grubości linii i rozmiarów czcionek, w zgodzie z makietami i skalą Vuetify.
- Sposób rysowania kratki (gradient CSS na `surface-variant`) i linii.
- Podział na pliki CSS (globalny plik stylów notesu vs `scoped`) i nazwy klas.
- Czy wiersz tabeli Planu na komputerze to `v-table`, czy siatka CSS.
