# Mealplan: dopasowanie produktów przez model (OpenRouter)

Data: 2026-11-11 · Status: do przeglądu · Rozszerza `2026-11-08-nutrition-mealprep-inventory-design.md` (§4.1 baza produktów) i `2026-11-03-recipe-parser-design.md`

## 1. Po co

Dziś składnik przepisu łączy się z produktem tylko wtedy, gdy jego nazwa jest **dokładnie** równa nazwie produktu albo jednemu z aliasów (`internal/nutrition/match.go`). Przez to „czerwona cebula” nie łączy się z niczym, bo w bazie jest „cebula czerwona”. Tak samo przepadają odmiany, literówki i dopiski. Właściciel zauważył też, że części produktów po prostu brakuje, a ręczne dodawanie mu się nie chce. Przy okazji w bazie zbierają się duplikaty (np. „pomidory” obok „pomidor”).

Właściciel chce wkleić w aplikacji własny klucz OpenRouter. Model ma wtedy przypisywać składniki do właściwych produktów i proponować brakujące produkty. Ma też wyszukiwać duplikaty, ale **nic nie zapisuje sam**: każda propozycja czeka na „Zatwierdź”.

**Kryteria sukcesu**
- Po wklejeniu przepisu prawie każdy składnik ma produkt albo propozycję produktu do zatwierdzenia jednym kliknięciem.
- Raz zatwierdzona nazwa (np. „czerwona cebula” → „cebula czerwona”) trafia do aliasów produktu. Kolejny przepis z tą nazwą dopasowuje się sam, bez pytania modelu.
- W bazie produktów nie powstają duplikaty. Te, które już są, da się scalić bez utraty danych.

## 2. Zakres

1. **Klucz OpenRouter w ustawieniach gospodarstwa.** Jest opcjonalny: bez klucza aplikacja działa dokładnie jak dziś.
2. **Dopasowanie przy imporcie przepisu.** Składniki bez dokładnego dopasowania idą do modelu, a wynik to propozycje do zatwierdzenia.
3. **„Sprawdź przepisy”.** Przegląd istniejących przepisów: składniki bez produktu albo z podejrzanym przypisaniem.
4. **„Sprawdź produkty”.** Przegląd bazy produktów: duplikaty do scalenia.
5. **Scalanie produktów.** Operacja w bazie, której wymaga punkt 4.
6. **Poprawka danych startowych.** „Cebula czerwona” staje się osobnym produktem (§6).

**Poza zakresem (non-goals)**
- Automatyczne zapisywanie czegokolwiek bez zatwierdzenia, także przy „pewnych” dopasowaniach.
- Inni dostawcy niż OpenRouter i modele lokalne. Interfejs klienta może to później umożliwić, ale tego nie budujemy.
- Liczenie i pokazywanie kosztów lub zużycia tokenów.
- Zadania w tle, kolejki i harmonogramy. Sprawdzanie uruchamia się tylko przyciskiem.
- Pobieranie wartości odżywczych z zewnętrznych baz. Model może je **zaproponować**, ale to propozycja do zatwierdzenia.

**Czego nie wolno zepsuć (anti-goals)**
- **Odmiany to nie duplikaty.** Czerwona cebula i cebula to dwa różne produkty, podobnie papryka czerwona i żółta. Scalane są wyłącznie te same rzeczy zapisane inaczej: liczba mnoga, odmiana przez przypadki, inny szyk, literówka, zdrobnienie („pomidory”, „cebuli”, „czerwona cebula” / „cebula czerwona”, „cebulka”). Tę zasadę model dostaje wprost w instrukcji.
- **Scalanie nie może gubić danych.** Przepisy, spiżarnia, ruchy spiżarni, dziennik jedzenia i lista zakupów przepinają się na produkt, który zostaje. Nic nie znika przez `ON DELETE CASCADE`.
- **Klucz nie wycieka.** API nigdy go nie zwraca: nie trafia do odpowiedzi, logów ani komunikatów błędów.
- **Model nie wymyśla ID.** Propozycja może wskazać tylko produkt z listy, którą model dostał, albo nowy produkt. Każda inna odpowiedź jest odrzucana.
- **Brak klucza albo awaria OpenRoutera nie blokuje** importu ani edycji. Wtedy działa dzisiejsze dokładne dopasowanie i pojawia się widoczny komunikat.

## 3. Ograniczenia

- Istniejący stos: Go, SQLite i Postgres (migracje przenośne na oba silniki), Vue 3 + Vuetify 3 + Pinia, OpenAPI jako kontrakt (`TestRoutesMatchOpenAPI`), sesja + CSRF, gospodarstwo jako tenant, interfejs po polsku, mobile-first.
- Produkt w bazie: `Name`, `NormalizedName`, `Per100g`, `PieceGrams`, `DensityGPerML`, `Aliases`. Alias jest unikalny w gospodarstwie (`PRIMARY KEY (household_id, alias)`).
- Tabele, które wskazują na produkt (do przepięcia przy scalaniu):
  - `ingredient.product_id` (SET NULL);
  - `food_log.product_id` (RESTRICT);
  - `pantry_item` (CASCADE, `UNIQUE (household_id, product_id, unit)`, więc ilości tej samej jednostki trzeba **zsumować**);
  - `pantry_movement` (CASCADE);
  - `shopping_item.product_id` (SET NULL);
  - `product_alias` (CASCADE).
- Nazwy produktów są w mianowniku liczby pojedynczej. Wyjątkiem są rzeczowniki bez liczby pojedynczej („płatki owsiane”). Tak samo jak w SKILL.md.
- Wyjście do sieci idzie tylko do stałego hosta `https://openrouter.ai/api/v1` (OpenAI-compatible `chat/completions`). Adres nie jest konfigurowalny przez użytkownika, więc nie ma ryzyka SSRF.

## 4. Jak to działa

### 4.1 Ustawienia

W ustawieniach gospodarstwa jest sekcja „Asystent produktów (OpenRouter)” z polami „Klucz API” i „Model”. Klucz i model ustawia **tylko administrator** gospodarstwa. Każdy członek może uruchamiać sprawdzanie i zatwierdzać propozycje, bo i tak może już edytować produkty.

- Po zapisaniu klucz jest pokazywany wyłącznie jako „ustawiony, kończy się na …abcd”, z przyciskami „Zmień” i „Usuń”.
- Przycisk „Sprawdź połączenie” wysyła minimalne zapytanie i pokazuje „działa” albo czytelny błąd (zły klucz, brak środków, nieznany model).
- Pod polem jest informacja, że nazwy składników i produktów (a przy imporcie także treść przepisu) są wysyłane do OpenRoutera.

### 4.2 Dopasowanie przy imporcie

Po sparsowaniu przepisu (URL albo tekst) najpierw działa dzisiejsze dokładne dopasowanie. **Do modelu trafiają tylko składniki, które się nie dopasowały.** Model dostaje:
- nazwy tych składników, z ilością i jednostką jako kontekst;
- listę produktów gospodarstwa: id, nazwa, aliasy.

Dla każdego składnika model zwraca jedną propozycję:
- **istniejący produkt** (id z listy) i krótkie uzasadnienie;
- albo **nowy produkt**: nazwa w mianowniku l. poj., wartości na 100 g (kcal, białko, węglowodany, tłuszcz), opcjonalnie waga sztuki i gęstość, aliasy;
- albo „nie wiem” (składnik zostaje bez produktu, jak dziś).

W edytorze przepisu, przy polu „Produkt” takiego składnika, widać propozycję, np. „Propozycja: cebula czerwona (nowy produkt)”, z przyciskami „Zatwierdź” i „Odrzuć”.
- Przy nowym produkcie „Zatwierdź” otwiera istniejący formularz produktu (`ProductDialog`) wypełniony propozycją. Wartości są oznaczone jako szacunek modelu, do poprawienia przed zapisem.
- Gdy model nie odpowie albo zwróci błąd, import kończy się normalnie, a w edytorze pojawia się jedna informacja, że podpowiedzi są niedostępne.

### 4.3 Zatwierdzenie dopasowania

Zatwierdzenie propozycji „istniejący produkt” albo zapisanie nowego produktu robi trzy rzeczy:
1. ustawia `productId` składnika;
2. **dodaje nazwę składnika (znormalizowaną) do aliasów produktu**, o ile nie jest już jego nazwą ani aliasem;
3. przy kolidującym aliasie (należy już do innego produktu) nie nadpisuje go, tylko to pokazuje.

Dzięki punktowi 2 następne przepisy z tą nazwą dopasują się bez modelu.

### 4.4 „Sprawdź przepisy”

Przycisk jest na liście przepisów. Przegląda przepisy gospodarstwa i zbiera składniki:
- **bez produktu**, które nie dopasowują się dokładnie;
- z **podejrzanym przypisaniem**, czyli takim, które model uznaje za odmianę albo inny produkt, np. składnik „czerwona cebula” przypięty do „cebula”.

Wynik to lista propozycji pogrupowana po nazwie składnika: ta sama nazwa w wielu przepisach jest jedną propozycją, która dotyczy wszystkich tych przepisów. Każda propozycja ma „Zatwierdź”, „Odrzuć” i możliwość wybrania innego produktu. Jest też „Zatwierdź wszystkie”, które nie obejmuje propozycji nowych produktów, bo te wymagają przejrzenia wartości. Zatwierdzenie działa jak w §4.3 dla każdego dotkniętego przepisu.

### 4.5 „Sprawdź produkty”

Przycisk jest na stronie produktów. Model dostaje listę produktów (nazwa, aliasy, wartości na 100 g) i zwraca grupy duplikatów. Każda grupa wskazuje produkt, który zostaje, i produkty do wchłonięcia, z uzasadnieniem.

Na liście grup są „Scal”, „Odrzuć” i możliwość zmiany produktu, który zostaje. Przed „Scal” widać, ile przepisów, pozycji spiżarni i wpisów dziennika zostanie przepiętych.

### 4.6 Scalanie

`MergeProducts(keep, absorbed[])` działa w jednej transakcji:
- przepina wszystkie odniesienia z §3 na `keep`;
- pozycje spiżarni tej samej jednostki sumuje (ilość, najbliższa data ważności, stały zapas, jeśli którakolwiek pozycja go miała);
- nazwy i aliasy wchłoniętych produktów dopisuje do aliasów `keep`;
- usuwa wchłonięte produkty;
- wartości odżywcze bierze z `keep`.

Działa bez modelu i jest dostępne także ręcznie: na stronie produktów opcja „Scal z…”.

## 5. Klucz i bezpieczeństwo

- Klucz jest przechowywany w bazie, w ustawieniach gospodarstwa. Szyfrowanie w spoczynku: **decyzja budującego, moja**. Jeśli szyfrowanie, to kluczem z pliku generowanego w katalogu danych przy pierwszym uruchomieniu albo ze zmiennej środowiskowej. Wymóg minimalny: klucz nigdy nie wychodzi przez API.
- Odpowiedź modelu jest traktowana jak niezaufane dane. Musi być poprawnym JSON-em zgodnym ze schematem (`response_format` json_schema, jeśli model to wspiera). Nazwy są przycinane i normalizowane, a liczby muszą być w rozsądnych zakresach (np. kcal 0–900 na 100 g, makro 0–100 g). Treść przepisu z internetu może zawierać polecenia dla modelu. Nie szkodzi to, bo wynik to wyłącznie propozycje do zatwierdzenia, a ID są sprawdzane.
- Limit czasu na zapytanie i limit rozmiaru odpowiedzi. Zapytania są dzielone na partie (np. do 40 składników albo 150 produktów na raz), żeby żadne pojedyncze żądanie HTTP z przeglądarki nie wisiało dłużej niż około 60 s, bo za Caddy są limity.
- Endpointy modelu mają ten sam limiter co import (`internal/httpapi/limiter`).

## 6. Poprawka danych startowych

W `internal/nutrition/seed/products.csv` aliasy „cebula czerwona” i „cebuli czerwonej” są dziś przy produkcie „cebula”. Zmiany:
- usunąć je stamtąd;
- dodać osobny produkt **„cebula czerwona”** (aliasy: odmiany i szyk „czerwona cebula”, „czerwonej cebuli”, „cebulka czerwona”), z wartościami z USDA dla red onion;
- „cebula biała” i „cebula żółta” zostają aliasami cebuli, bo to ta sama zwykła cebula.

Seed działa raz na gospodarstwo, więc dla istniejących gospodarstw potrzebna jest migracja. Usuwa wymienione aliasy z produktu nazwanego „cebula” (jeśli tam są) i dodaje produkt „cebula czerwona”, jeśli gospodarstwo nie ma produktu o tej nazwie ani aliasie. Składników nie przepina: zrobi to „Sprawdź przepisy”.

## 7. Rozstrzygnięcia (po przeglądzie)

Odpowiedzi wynikające z kodu albo moje decyzje (oznaczone „moja decyzja”):

1. **Import nie zapisuje przepisu.** Szkic trafia do edytora niezapisany (`imports.setDraft` → `recipe-new`), a składniki nie mają jeszcze id. Propozycje przychodzą więc razem z odpowiedzią importu, jako pole przy każdym składniku szkicu (analogicznie do `matchedProductId`), i żyją w stanie edytora. „Zatwierdź” ustawia `productId` w formularzu i od razu dopisuje alias (osobne wywołanie, §4.3). Zapisanie przepisu utrwala przypisanie. „Odrzuć” tylko chowa propozycję.
2. **„Sprawdź przepisy”** (moja decyzja): propozycje są nietrwałe (odpowiedź API, stan w przeglądarce). Zatwierdzenie idzie przez nowy endpoint zbiorczy „przypisz produkt do składników o tej nazwie w tych przepisach + dodaj alias”, w jednej transakcji. Ustawia tylko `ingredient.product_id`, nie przepisuje całych przepisów i nie zmienia ich `updated_at`.
3. **Normalizacja aliasu** jest ta sama co w `match.go`: `units.NormalizeName` (małe litery, zwinięte spacje, polskie znaki zostają). Alias zapisywany jest w tej formie. Kolizja z aliasem innego produktu (moja decyzja): komunikat „„X” jest już aliasem produktu Y”, alias nie jest dodawany, przypisanie składnika i tak się zapisuje. Bez propozycji scalenia.
4. **Scalanie** (moja decyzja):
   - pozycje spiżarni o różnych jednostkach zostają osobnymi pozycjami produktu `keep`, bez przeliczania jednostek;
   - przy tej samej jednostce suma, a `pantry_movement` przepinany jest tylko po `product_id`, bo ruchy wskazują produkt, a nie pozycję;
   - flaga `Staple` istnieje (`domain.PantryItem.Staple`), staple wygrywa, a staple ma ilość `nil`;
   - kolejność: najpierw przepięcia (w tym `food_log`, który ma RESTRICT), na końcu usunięcie;
   - nazwa lub alias wchłanianego produktu należący już do trzeciego produktu jest pomijany i wymieniany w wyniku scalenia, a scalanie się nie przerywa.
5. **Ustawienia:** endpoint `GET/PUT /api/v1/household/settings` już istnieje, a ustawienia są kolumnami tabeli `household`. Klucz i model to nowe kolumny tam, na tym samym endpoincie albo na osobnym `PUT /household/openrouter`, do wyboru budującego. Klucz jest wspólny dla gospodarstwa. „Administrator” to `RoleAdmin`. Funkcji eksportu nie ma: wymóg dotyczy odpowiedzi API i logów.
6. **Migracja cebuli:** dopasowanie po `normalized_name = 'cebula'`. Gospodarstwo, które cebulę usunęło lub przemianowało, ma tylko dodany produkt „cebula czerwona” (jeśli nie ma go jako nazwy lub aliasu). Aliasy usuwane są tylko wtedy, gdy należą do produktu o tej nazwie.
7. **Partie i czas** (moja decyzja): partiami steruje przeglądarka. Endpoint przyjmuje jedną partię i zwraca jej wynik, a interfejs pokazuje postęp „partia 2 z 5” i pozwala przerwać. Serwer ma twardy limit czasu na jedno wywołanie modelu (np. 50 s).
8. **Duplikaty między partiami** (moja decyzja): przed wysłaniem do modelu produkty są sortowane po nazwie i grupowane w kandydatów lokalnie (wspólny pierwszy rdzeń nazwy, np. pierwsze 4 litery pierwszego słowa po normalizacji, albo wspólne słowo). Partia to zbiór kandydatów, a nie kolejne 150 produktów. Pary rozdzielone mimo to są akceptowalne, bo „Sprawdź produkty” można uruchomić ponownie.

## 8. Co zostawiam budującemu

- Kształt endpointów i tabel na propozycje: czy propozycje są trzymane w bazie, czy tylko zwracane i trzymane w przeglądarce do zatwierdzenia. Warunek: zamknięcie strony w trakcie przeglądu nie może niczego zepsuć.
- Domyślny model. Ma być tani i szybki, z obsługą JSON-owego wyjścia, i dać się zmienić w ustawieniach. Przed wyborem sprawdzić aktualną listę modeli OpenRoutera.
- Treść instrukcji dla modelu, z dwoma obowiązkowymi elementami: zasadą odmiany vs duplikat z §2 i zasadą mianownika l. poj. z §3.
- Szczegóły wyglądu listy propozycji. Mobile-first, spójnie z resztą aplikacji (§ „Wygląd i dostępność” w CHANGELOG).
- Testy: klient OpenRouter za interfejsem z fałszywą implementacją w testach. Żaden test nie łączy się z prawdziwym OpenRouterem.

## 9. Dokumentacja

`docs/import.md` (dopasowanie przy imporcie), `docs/nutrition.md` (scalanie, aliasy), `docs/security.md` (klucz, dane wysyłane do OpenRoutera), `docs/api.md` i OpenAPI, CHANGELOG.
