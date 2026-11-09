# Changelog

## Unreleased

### Wydajność

- Serwer wysyła pliki aplikacji skompresowane (gzip, jeśli przeglądarka go przyjmuje) i z `ETag`, więc `index.html` i pliki bez skrótu w nazwie są tylko sprawdzane, a nie pobierane od nowa.
- Ikony są rysowane jako SVG tylko z 39 używanych (`web/src/icons.ts`, pilnowane testem) zamiast całej czcionki Material Design Icons: znikają czcionka (~400 KB woff2, ~3 MB innych formatów w binarce) i ~300 KB CSS.
- Widoki poza „Dziś” i logowaniem ładują się przy pierwszym wejściu. Karta otwarta przed wdrożeniem przeładowuje się raz, gdy nie znajdzie starego pliku widoku.

### Meal prep

- Zakładka „Meal prep": sesje gotowania z daniami (przepis i liczba porcji), przydział porcji do wpisów planu („z meal prepu"), wspólna zamrażalka gospodarstwa („z zamrażalki", „❄ Zamroź", ręczne dodawanie) i odpis porcji zjedzonych lub wyrzuconych (`/prep/*`, `/freezer*`). Sesja należy do tygodnia, na który się gotuje: weekendowa do następnego, inne dni do bieżącego (do zmiany przy sesji).
- „Ugotowane" na sesji zdejmuje ze spiżarni składniki wszystkich porcji dań naraz; późniejsze dodanie, usunięcie lub zmiana porcji dania przelicza spiżarnię, a usunięcie ugotowanej sesji oddaje zapasy. „Zrobione" na wpisach z meal prepu nigdy nie rusza spiżarni. Usunięcie sesji, dania lub pozycji zamrażalki z przydziałami jest odrzucane (`in_use`); kopiowanie tygodnia pomija wpisy z meal prepu. Lista zakupów liczy składniki na wszystkie porcje dań sesji.
- Wpisy planu mają `kind` (`meal`, `prep`, `out`), `prepDishId`, `freezerItemId` i `outValues`; nowy rodzaj „Na mieście" z opcjonalnym opisem i wartościami na osobę, bez kucharza i bez zakupów.
- Ustawienie gospodarstwa „Porcji na posiłek" (`/household/settings`, 1-20, domyślnie 2) na ekranie Domownicy; ekran „Dziś" przypomina o nieprzydzielonych porcjach sesji starszej niż 4 dni. Opis: `docs/mealprep.md`.

### Spiżarnia

- Spiżarnia gospodarstwa (`/pantry`, `/pantry/review`): pozycje wskazują produkty (nazwa może rozwiązać się przez alias, nieznana tworzy produkt), ilości w jednostkach bazowych (kg jako g, l jako ml), ilość 0, stały zapas bez ilości, data ważności, przyciski +/- z opóźnionym, szeregowanym zapisem i miesięczny przegląd z przypomnieniem na ekranie „Dziś" po ponad 30 dniach.
- „Zrobione" na posiłku z przepisu zdejmuje składniki przeskalowane do porcji, w tej samej jednostce bazowej lub przeliczalnej (łyżka 15 ml, łyżeczka 5 ml, szklanka 250 ml; na gramy tylko z gęstością produktu, sztuki tylko z wagą sztuki) i nie poniżej 0; zapisuje, ile zdjęto, a cofnięcie zwraca dokładnie to. Ponowny zapis nie odejmuje drugi raz, a usunięcie zrobionego wpisu nie zwraca zapasu. Błąd `pantry_failed` oznacza zapisany posiłek bez aktualizacji spiżarni (ponowny zapis naprawia).
- Lista zakupów pomija zrobione posiłki i odejmuje zapas per produkt i jednostka bazowa: pozycje `haveQuantity` („masz w domu", „kup X (masz Y)"), stałe zapasy są pomijane. „Przenieś kupione do spiżarni" (`POST /shopping/move-to-pantry`, pole `moved`) dodaje odhaczone pozycje raz. Opis: `docs/pantry.md`.
- Ograniczenia: bez gęstości lub wagi sztuki produktu objętość i sztuki nie pomniejszają zapasu w gramach, brak rezerwacji zapasu między tygodniami, dwie nazwy jednego produktu to dwa wiersze listy, dowolne jednostki nigdy nie są dopasowywane.
- Import: linia `Wartości na porcję` z „(w tym nasycone …)", „z czego" lub „of which" przy tłuszczu nie gubi już wartości tłuszczu (segment jest ucinany zamiast pomijany).

### Wygląd

- Nowa domyślna paleta „Mealplan" (zieleń szałwiowa i ciemna zieleń, jasny i ciemny motyw) oraz logo: napis w nagłówku i na ekranach logowania, konfiguracji i zaproszenia (wariant dobierany do tła), ikona na telefonie i favicon zależny od motywu systemu. Przyciski mają ciemniejszą zieleń (#4F6B3F) niż zieleń marki (#7B9A68), żeby biały tekst spełniał kontrast WCAG AA; zieleń marki jest kolorem akcentu (paski postępu, ikony). Zapisany wybór palety użytkownika nie zmienia się.

### Wartości odżywcze

- Przepis ma wartości na porcję (kcal, białko, węglowodany, tłuszcz) z osobnym źródłem każdej wartości: wpisane w przepisie, wyliczone ze składników przez bazę produktów gospodarstwa albo brak. Wyliczenie oznacza „przybliżone", gdy składnik nie ma produktu lub jego ilości nie da się przeliczyć na gramy.
- Produkty gospodarstwa (`/products`, wartości na 100 g, gęstość, waga sztuki, aliasy). Dane startowe: ok. 197 produktów z USDA FoodData Central SR Legacy (kwiecień 2018, CC0 1.0), generator w `internal/nutrition/seed/gen`; kopiowane do gospodarstwa przy pierwszym użyciu. Znane ograniczenie: brak suszonych przypraw i wody, więc przepisy z nimi są „przybliżone", dopóki ich nie dodasz.
- Konto: włączenie śledzenia, dzienny cel (każda wartość opcjonalna), dziennik jedzenia (produkt i ilość w g lub szt) i podsumowanie dnia z ostrzeżeniami o brakujących danych (`/me/nutrition`, `/me/nutrition/days`, `/me/food-log`).
- Import: opcjonalna linia `Wartości na porcję: …` w tekście i pole `nutrition` w JSON-LD; umiejętność Claude `mealplan-recipe-format` zna nową linię. Opis: `docs/nutrition.md`, `docs/import.md`.

### Notatki do składników

- Składnik przepisu ma opcjonalną notatkę (`note`, do 200 znaków), np. „można zastąpić jogurtem greckim". Jest zapisywana razem z przepisem (migracja `0004`), a import zapisuje ją zamiast odrzucać; żądania bez `note` nadal działają. Długie linie zaczynające się od ilości (11-30 słów) są teraz składnikami, a notatki dłuższe niż 200 znaków są ucinane z ostrzeżeniem.
- Edytor przepisu ma przy każdym składniku pole „Notatka” (do 200 znaków); wartość z importu lub z zapisanego przepisu jest w nim widoczna i zapisuje się razem z przepisem.
- Ekran importu ma kartę „Używasz Claude?” z przyciskiem pobierania umiejętności `mealplan-recipe-format` (`/claude-skill/SKILL.md`, plik kanoniczny `web/public/claude-skill/SKILL.md`).

### Import przepisów

- Import przepisu z linku (`POST /recipes/import/url`) lub wklejonego tekstu (`POST /recipes/import/text`, do 100 KB). Źródła strony: JSON-LD, microdata, a w ostateczności tekst strony (min. 2 składniki i 1 krok, z ostrzeżeniem). Parser linii składnika (ilość, jednostka, nazwa, notatka; `ok`/`guess`/`unparsed`). Import niczego nie zapisuje.
- Podgląd w edytorze przepisu: niepewne wiersze (`guess`, `unparsed`) są podświetlone, linia `unparsed` zachowuje oryginalny tekst jako nazwę, notatka składnika jest zapisywana razem ze składnikiem. Wykrywanie duplikatów (ten sam adres źródła lub tytuł) z wyborem „Zaktualizuj istniejący" albo „Dodaj jako nowy".
- Ochrona SSRF: blokada zakresów prywatnych, loopback, link-local i `100.64.0.0/10`, kontrola w dialerze po rozwiązaniu DNS (także przekierowania i rebinding), porty 80/443, bez proxy z otoczenia, limity 2 MB, 10 s i 5 przekierowań. Limit 10 importów z linku na minutę na użytkownika (`429 rate_limited`). Nowe kody błędów `import_blocked`, `import_unparsable`, `import_failed`, `import_timeout`.
- Nowa zależność: `golang.org/x/net` (parser HTML i dekodowanie kodowań stron). Opis: `docs/import.md`.
- Rozszerzona blokada adresów (NAT64, 6to4, Teredo, `::/96`, `fec0::/10`, `198.18.0.0/15`, `240.0.0.0/4`, `192.0.0.0/24`), limity pracy parsera (linie, składniki, kroki), usuwanie znaków sterujących ze szkicu, ilość po nazwie składnika („czosnek 2 ząbki") i przymiotniki przed jednostką.
- Uwaga: jednostki zapisane w istniejących przepisach przed tą zmianą (np. „łyżki") nie są przepisywane i na liście zakupów nie scalą się z nowymi formami kanonicznymi („łyżka"), dopóki przepis nie zostanie zapisany ponownie.

### Nowy wygląd

- Ekran „Dziś" jako strona startowa: najbliższy niegotowy posiłek dnia, pasek dni tygodnia, lista posiłków z kucharzem i przycisk „Zrobione" (`plan_entry.done`, migracja 0003, pole `done` w API; kopiowanie tygodnia zeruje stan). Siatka tygodnia przeniesiona na `/plan`.
- Szybkie dodawanie posiłku z podpowiedziami przepisów, dniem, posiłkiem i kucharzem.
- Nawigacja: ciemny pasek boczny na komputerze, dolny pasek na telefonie; ustawienia (kolory, motyw, domownicy, wylogowanie) w menu awatara.
- Material 3: duże zaokrąglenia, płaskie powierzchnie z cienką ramką, dolny arkusz zamiast okna dialogowego na telefonie, puste stany z ilustracją, płynne przejścia.
- Palety kolorów (Klasyczny, Śliwka, Ziemia, Granat), domyślnie Ziemia.

### Etap 2: rdzeń

- Przepisy (składniki, kroki, tagi, `sourceUrl` tylko http/https), tygodniowy plan (wpis: przepis albo tekst wolny, slot, porcje, kucharz), kopiowanie tygodnia, lista zakupów generowana z planu z ręcznymi pozycjami, jednostki i normalizacja nazw.
- Zarządzanie członkami: lista, zmiana roli, usuwanie (kasuje sesje członka w gospodarstwie i czyści go jako kucharza); ostatni administrator chroniony (`409 last_admin`). Włączone klucze obce w SQLite.
- UI: widoki przepisów, planu tygodnia, listy zakupów i członków.
- Rozstrzygnięcia: przepis użyty w planie nie da się usunąć (`409 in_use`); kopiowanie tygodnia tylko na pusty tydzień (`409 conflict`); pozycje wygenerowane są tylko do odhaczania, edytować i usuwać można ręczne; regeneracja zachowuje pozycje ręczne oraz `id` i `checked` niezmienionych pozycji (zmiana ilości zeruje odhaczenie); generowanie z zakresem dni zastępuje wygenerowane pozycje całego tygodnia pozycjami z zakresu; tydzień identyfikuje poniedziałek `YYYY-MM-DD`; odejmowanie zapasów podpięte, ale nieaktywne do Etapu 3.

### Etap 1

- Etap 1: konfiguracja, baza z migracjami (SQLite, pod Postgres), logowanie argon2id, sesje, CSRF, limity prób, zaproszenia, API z OpenAPI, powłoka UI Vue + Vuetify, osadzony frontend, Dockerfile, CI.
- Poprawki po przeglądzie końcowym: sprawdzenie konfiguracji przed haszowaniem, ograniczenie współbieżności argon2, odrzucanie zbyt długich loginów, polskie komunikaty błędów i podpowiedź loginu, poprawki dokumentacji (zapraszanie, blokady, README) i CI. Znana, nieużywana luka GO-2026-5932 (`golang.org/x/crypto/openpgp`, brak poprawki, pakiet nie jest importowany).
- Toolchain Go 1.26 i aktualne zależności (m.in. pgx v5.12.0, modernc.org/sqlite v1.61.1, golang.org/x/crypto v0.58.0, golang.org/x/text v0.43.0); `govulncheck` nie zgłasza podatności w używanym kodzie.
