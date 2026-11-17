# Mealplan: widok przepisu i przeliczanie porcji

Data: 2026-11-17 · Status: do przeglądu · Punkt 2 z `docs/roadmap.md`

## 1. Po co

Dziś kliknięcie przepisu otwiera od razu pełny formularz edycji (`RecipeEditView`): każdy składnik zajmuje kilka wierszy z polami ilości, jednostki, notatki i produktu. Kiedy gotujesz z telefonu, to za dużo. Do tego, gdy w planie jest 6 porcji, a przepis jest na 2, ilości trzeba przeliczać w głowie.

Budujemy prosty widok przepisu do czytania przy gotowaniu, z przeliczaniem ilości składników na wybraną liczbę porcji.

**Kryteria sukcesu**
- Na telefonie przepis czyta się bez przewijania formularzy: jedna linia na składnik, ponumerowane kroki.
- Z ekranu „Dziś” przycisk „Przepis” otwiera widok od razu przeliczony na liczbę porcji z planu.
- Przeliczone ilości wyglądają tak, jak napisałby je człowiek („1,5 łyżki”, „330 g”), a nie „1,4999 łyżki” ani „333,33 g”.

## 2. Zakres

1. Nowy widok przepisu pod `recipes/:id`.
2. Edycja przenosi się pod `recipes/:id/edit`.
3. Przeliczanie porcji w widoku, tylko na ekranie, bez zmiany przepisu.
4. Wejście z ekranu „Dziś” z liczbą porcji z planu.

**Poza zakresem (non-goals)**
- Zapisywanie przeliczonej liczby porcji jako nowych porcji bazowych. Kto chce, zmienia je w „Edytuj”.
- Pomoce do gotowania: odhaczanie składników i kroków, ekran, który nie gaśnie. To kolejny etap.
- Zdjęcia (punkt 3 roadmapy). Widok ma zostawić na nie miejsce u góry, ale ich nie pokazuje.
- Zmiany w backendzie, API i bazie. Wszystko, czego widok potrzebuje, zwraca już `GET /recipes/{id}`.
- Linki do przepisu z planu tygodnia, meal prepu i zakupów. Dziś ich nie ma i ten etap ich nie dodaje.
- Przeliczanie kalorii: wartości odżywcze są na porcję, więc nie zależą od liczby porcji.

**Anty-cele (porażka, nawet jeśli działa)**
- Widok, w którym przy liczbie porcji równej bazowej ilości różnią się od zapisanych w przepisie (np. zaokrąglone „125 g” do „125 g”, ale „0,3 szklanki” do „0,25”).
- Przeliczanie, które po cichu zmienia przepis albo plan.

## 3. Podejścia

| | Podejście | Za | Przeciw |
|---|---|---|---|
| **A (wybrane)** | Osobna strona `RecipeView` pod `recipes/:id`, edycja pod `recipes/:id/edit` | Czysty podział czytania i edycji; „wstecz” na telefonie działa naturalnie; link do przepisu można wysłać domownikowi; porcje w adresie przeżywają odświeżenie | Trzeba poprawić kilka linków |
| B | Tryb czytania w `RecipeEditView`, przełączany „Edytuj” | Bez zmian w linkach | Plik ma 337 linii i mieszałby dwie rzeczy; „wstecz” nie cofa z edycji do czytania |
| C | Okno dialogowe nad listą lub ekranem Dziś | Najmniej nawigacji | Brak własnego adresu; „wstecz” zamyka stronę; nie da się wysłać linku |

Właściciel wybrał A.

## 4. Projekt

### 4.1 Adresy i nawigacja

| Adres | Nazwa trasy | Widok |
|---|---|---|
| `recipes/:id` | `recipe` (nowa) | `RecipeView` (nowy) |
| `recipes/:id/edit` | `recipe-edit` (nazwa bez zmian, nowa ścieżka) | `RecipeEditView` |
| `recipes/new` | `recipe-new` | `RecipeEditView`, bez zmian |

Nazwa `recipe-edit` zostaje, bo zależy od niej `importDraft.ts` (trasy, które korzystają ze szkicu importu). Trasa `recipes/:id/edit` musi być zdefiniowana tak, żeby `recipes/new`, `recipes/import` i `recipes/check` dalej trafiały do swoich widoków.

Zmiany linków:
- Lista przepisów (`RecipesView`): wiersz prowadzi do widoku (`recipe`).
- Ekran Dziś, przycisk „Przepis”: prowadzi do widoku z `?servings=<porcje z wpisu planu>`.
- Import duplikatu (`ImportView`): zostaje przy `recipe-edit`, bo tam scala się szkic z istniejącym przepisem.
- Formularz edycji istniejącego przepisu: „Zapisz” wraca do widoku tego przepisu z komunikatem „Zapisano przepis „…””, a nie do listy. Nowy przepis po zapisie też otwiera swój widok. „Usuń” wraca do listy, jak dziś. „Anuluj” w edycji istniejącego przepisu wraca do jego widoku. „Edytuj” nie przenosi `?servings`, a powrót z edycji otwiera widok na porcjach bazowych. *(decyzja projektanta: po edycji naturalnie wraca się do tego, co się czytało)*
- Widok ma przycisk powrotu do listy przepisów.

### 4.2 Co pokazuje widok

Od góry, w jednej kolumnie (na komputerze z ograniczoną szerokością, żeby linie nie były za długie):

```
←  Zupa ogórkowa                      [Edytuj]
   zupa · obiad                         (tagi)
   320 kcal na porcję · B 12 g · W 30 g · T 15 g

   Porcje   [ − ]  6  [ + ]
   Przeliczone z 2 porcji · Przywróć

   Składniki
   600 g ziemniak
   1,5 l bulion warzywny
   3 ogórki kiszone  (starte)
   sól  do smaku

   Przygotowanie
   1. Obierz ziemniaki i pokrój w kostkę.
   2. …

   Źródło: kwestiasmaku.com ↗
```

- **Tytuł, tagi, przycisk „Edytuj”.**
- **Wartości odżywcze na porcję** z `nutritionPerPortion`. Gdy są przybliżone (`approximate`), przed kaloriami stoi „ok.”. Pokazywane są tylko znane wartości; gdy nie ma żadnej, wiersza nie ma. Gdy domownik ma wyłączone śledzenie kalorii, wiersza też nie ma, tak jak na ekranach Dziś i Plan. *(decyzja projektanta)*
- **Porcje:** przyciski − i + oraz liczba. Zakres 1–100, krok 1, jak `maxServings` na serwerze. Gdy liczba różni się od bazowej, pod spodem „Przeliczone z N porcji” i „Przywróć” (wraca do bazowej).
- **Składniki:** jedna linia każdy: ilość, jednostka w odpowiedniej formie (`unitWord`), nazwa, a notatka po niej w kolorze drugorzędnym. Składnik bez ilości: sama nazwa i notatka.
- **Kroki:** lista numerowana.
- **Źródło:** link z domeną, otwierany w nowej karcie (`rel="noopener noreferrer"`). Gdy brak źródła, nic.
- Stany: ładowanie, błąd („nie znaleziono przepisu” przy 404, ogólny przy innych), jak w innych widokach.
- Wszystkie teksty w `locales/pl.ts` i `en.ts`.

### 4.3 Liczba porcji w adresie

- Widok czyta `?servings=` z adresu. Brak, nieliczba, poza zakresem 1–100: używa porcji bazowych przepisu.
- Zmiana − lub + aktualizuje adres przez `router.replace` (bez nowego wpisu w historii), więc odświeżenie zachowuje porcje, a „wstecz” wraca tam, skąd przyszedłeś.
- Gdy liczba wraca do bazowej, parametr znika z adresu.

### 4.4 Przeliczanie i zaokrąglanie

Osobny moduł `web/src/scale.ts` z testami, podobnie jak `polish.ts`. Bez zależności od Vue.

- Współczynnik = wybrane porcje / porcje bazowe.
- **Gdy współczynnik = 1, ilości są pokazywane dokładnie tak, jak są zapisane.** Zaokrąglanie działa tylko na przeliczonych ilościach.
- Składnik bez ilości (`quantity: null`) zostaje bez zmian.
- Zaokrąglanie przeliczonej ilości zależy od jednostki *(decyzja projektanta, do skorygowania przez właściciela)*:

| Jednostka | Zaokrąglenie | Minimum |
|---|---|---|
| g, ml, poniżej 10 | do 0,5 | 0,5 |
| g, ml, 10–99 | do 1 | |
| g, ml, 100–999 | do 10 | |
| g, ml, od 1000 | zamiana na kg / l, potem jak kg / l | |
| kg, l, poniżej 1 | zamiana na g / ml, potem jak g / ml | |
| kg, l, od 1 | do 0,05 | |
| łyżka, łyżeczka, szklanka | poniżej 1 do 0,25; od 1 do 0,5 | 0,25 |
| szczypta | do 1 | 1 |
| szt, bez jednostki i pozostałe jednostki kuchenne (ząbek, plaster, opakowanie, puszka, pęczek, garść, kostka, główka, kromka) | do 0,5 | 0,5 |
| inne (nieznane) jednostki | poniżej 10 do 0,1; od 10 do 1 | |

- Przedział wybiera się przed zaokrągleniem, a po zaokrągleniu wynik sprawdza się jeszcze raz: 998 g zaokrągla się do 1000 g i pokazuje jako „1 kg”, 0,999 kg jako „1 kg”, 0,9 łyżki jako „1 łyżka”. *(decyzja projektanta)*
- Przykłady oczekiwanych wyników (przepis na 2 porcje): 125 g × 3 = 375 g → „380 g”; 110 g × 3 = 330 g → „330 g”; 4,5 g × 3 = 13,5 g → „14 g”; 1 łyżka × 1,5 → „1,5 łyżki”; 1 łyżeczka ÷ 4 → „0,25 łyżeczki”; 3 jajka (bez jednostki) ÷ 2 → „1,5”; 1 szczypta ÷ 2 → „1 szczypta”; 0,5 kg ÷ 2 → „250 g”.
- Nieznane jednostki mają minimum 0,1. *(decyzja projektanta)*
- Liczby są formatowane jak w reszcie aplikacji (`fmtNumber`: przecinek po polsku, kropka po angielsku), a jednostka dostaje formę zgodną z liczbą („1,5 łyżki”, „2 ząbki”, „5 łyżek”).
- Wejście jednostek: tak, jak jest zapisane w przepisie (formy podstawowe, np. `łyżka`, `szt`, `g`). Moduł nie normalizuje innych zapisów; nieznana jednostka idzie wierszem „inne”.

### 4.5 Testy

- `scale.test.ts`: współczynnik 1 zwraca dokładnie zapisane ilości; każdy wiersz tabeli z §4.4, w tym granice (9,9 / 10 / 999 / 1000 g, 0,99 kg) i minima; `quantity: null`.
- Test widoku (jak `TodayView.test.ts`): wyświetla składniki w jednej linii, − i + zmieniają ilości i adres, `?servings=` z adresu jest użyte, wartości spoza zakresu są ignorowane, „Przywróć” wraca do bazowych.
- Testy linków: lista przepisów prowadzi do widoku, przycisk na Dziś przekazuje `servings`, zapis w edytorze wraca do widoku.
- Istniejące testy `RecipeEditView` poprawione pod nową ścieżkę.

### 4.6 Dokumentacja

- `CHANGELOG.md`: nowa sekcja w „Unreleased”.
- `docs/roadmap.md`: punkt 2 oznaczony jako zrobiony, z decyzją o niezapisywaniu porcji.

## 5. Zostawione wykonawcy

- Dokładny wygląd (rozmiary, odstępy, ikony) w stylu reszty aplikacji (Vuetify, istniejące palety, kontrast WCAG AA pilnowany testem).
- Nazwy kluczy tłumaczeń i podział na komponenty.
- Jak pokazać notatkę składnika: w nawiasie czy samym kolorem.
