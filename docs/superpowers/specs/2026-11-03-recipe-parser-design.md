# Mealplan: parser przepisów (import z linku i wklejonego tekstu)

Data: 2026-11-03 · Status: do przeglądu · Rozszerza §7.3 specu `2026-11-01-mealplan-design.md`

## 1. Cel

Dodanie przepisu ma zajmować sekundy, nie minuty pisania. Użytkownik wkleja link lub tekst z dowolnego źródła, aplikacja zamienia go na strukturę (tytuł, porcje, składniki z ilością i jednostką, kroki), a użytkownik sprawdza wynik w podglądzie i zapisuje.

**Kryteria sukcesu**
- Typowy przepis z polskiego serwisu kulinarnego (link) importuje się bez ręcznych poprawek.
- Przepis wklejony jako zwykły tekst daje użyteczny szkic; linie, których parser nie rozumie, są wyraźnie oznaczone, a nie gubione.
- Nic nie jest zapisywane bez zatwierdzenia w podglądzie.
- Parser działa offline, bez zewnętrznych usług i kluczy API.

**Poza zakresem:** import plików `.md` (format bez zmian, ten sam pipeline, osobny etap), zdjęcia i OCR, fallback LLM (przewidziany tylko jako punkt rozszerzenia, §3).

## 2. Decyzje

| Decyzja | Wybór |
|---|---|
| Podejście | Deterministyczny parser, testowalny tablicowo; bez LLM w tym etapie |
| Wspólny wynik | Każde źródło daje `Draft`; zapis idzie istniejącym `POST /recipes` |
| Rozszerzalność | Interfejs `Parser`; opcjonalny fallback LLM (własny klucz, domyślnie wyłączony) może być dodany później bez zmian w pipeline |
| Wynik niepewny | Linia składnika z `confidence: unparsed` zachowuje tekst oryginalny i jest podświetlona w podglądzie |
| Zapis | Brak nowych endpointów zapisu |

## 3. Architektura (`internal/importer`)

```go
type Source struct {
    Kind string // "url" | "text"
    URL  string
    Text string
}

type Draft struct {
    Title       string
    Servings    int // 0 = nieznane
    SourceURL   string
    Tags        []string
    Ingredients []IngredientLine
    Steps       []string
    Warnings    []string
}

type IngredientLine struct {
    Raw        string
    Name       string
    Quantity   *float64
    Unit       string
    Note       string // np. "posiekana"
    Confidence string // "ok" | "guess" | "unparsed"
}

type Parser interface {
    Parse(ctx context.Context, s Source) (Draft, error)
}
```

Pliki:
- `fetch.go`: pobieranie z ochroną SSRF (§5).
- `jsonld.go`, `microdata.go`: ekstrakcja strukturalna ze strony.
- `text.go`: sekcje i klasyfikacja linii dla czystego tekstu.
- `ingredient.go`: parser pojedynczej linii składnika (rdzeń).
- `draft.go`: typy i walidacja limitów (te same, co w `recipe`: tytuł 1–200, porcje 1–100, składniki ≤200, kroki ≤100 po ≤2000 znaków, tagi ≤20 po ≤40).

Parser ilości z §7.3 pierwszego specu to `ingredient.go` korzystający z `internal/units` (`CanonUnit`, `NormalizeName`); aliasy jednostek są rozszerzane w `units`, nie duplikowane.

## 4. Parser linii składnika

Wejście: `"2 łyżki oliwy z oliwek, podgrzanej"`. Wyjście: `Quantity 2, Unit "łyżka", Name "oliwa z oliwek", Note "podgrzana"`.

**Ilość**
- liczby całkowite i dziesiętne (`2`, `1,5`, `1.5`), ułamki (`1/2`), ułamki unicode (`½`, `¼`), mieszane (`1 ½`, `1 1/2`);
- zakres (`2-3`, `2–3`, `2 do 3`) daje wartość górną;
- słowa: `pół` = 0,5, `ćwierć` = 0,25;
- brak ilości (`sól`, `do smaku`, `szczypta`, `trochę`) daje `Quantity = nil`.

**Jednostka**
- rozpoznawane formy polskie i skróty: `g`, `dag`, `kg`, `ml`, `l`, `łyżka/łyżki/łyżek`, `łyżeczka/…`, `szklanka/szklanki/szkl.`, `ząbek/ząbki`, `plaster/plasterki`, `szczypta`, `opakowanie/op.`, `puszka`;
- bez jednostki i z rzeczownikiem policzalnym (`3 jajka`) daje `szt`;
- jednostki są kanonizowane przez `units.CanonUnit`.

**Nazwa i notatka**
- nazwa to reszta po ilości i jednostce, bez przyimka `z`/`ze` na początku, w formie jak w tekście (bez lematyzacji);
- tekst po przecinku i w nawiasach trafia do `Note`;
- gdy nie da się wskazać nazwy, linia dostaje `unparsed` i zachowuje `Raw`.

**Confidence**
- `ok`: ilość, jednostka (lub `szt`) i nazwa rozpoznane;
- `guess`: ilość lub jednostka wydedukowana (np. `3 jajka` → `szt`, zakres → górna wartość);
- `unparsed`: nic pewnego.

## 5. Źródła

### Link

1. Pobranie z ochroną SSRF: tylko http/https; po rozwiązaniu DNS odrzucane adresy prywatne, loopback i link-local; ta sama kontrola przy każdym przekierowaniu (max 5); limit rozmiaru 2 MB i czas 10 s; `User-Agent: Mealplan`. Surowy HTML nigdy nie jest renderowany.
2. JSON-LD `Recipe`, także w `@graph` i tablicach: `name`, `recipeYield` (liczba, tekst lub tablica), `recipeIngredient`, `recipeInstructions` (tekst, `HowToStep`, `HowToSection`), `keywords`/`recipeCategory` jako tagi (czasy przygotowania pomijane w tej wersji).
3. Microdata `itemtype=…/Recipe`, jeśli brak JSON-LD.
4. Ostatnia deska ratunku: tekst strony (bez `script`, `style`, `nav`, `footer`) przepuszczony przez parser tekstu z ostrzeżeniem `Warnings: "Dane strukturalne nie znalezione, wynik oparty na tekście strony"`.
5. Każda linia składnika ze źródła strukturalnego przechodzi przez parser linii (§4).

### Wklejony tekst

- Nagłówki sekcji (PL i EN, bez rozróżniania wielkości liter, z dwukropkiem lub bez, z `#`): `Składniki`, `Ingredients` oraz `Przygotowanie`, `Sposób wykonania`, `Wykonanie`, `Kroki`, `Method`, `Instructions`, `Directions`.
- Bez nagłówków: linia zaczynająca się od ilości lub wypunktowania to składnik; linia numerowana lub dłuższe zdanie to krok.
- Tytuł: pierwsza niepusta linia, jeśli nie jest nagłówkiem sekcji ani składnikiem; nagłówek markdown `# …` ma pierwszeństwo.
- Porcje: `Porcje: 4`, `4 porcje`, `dla 4 osób`, `serves 4`; brak daje `0` i ostrzeżenie.
- Numerację kroków (`1.`, `1)`, `Krok 1:`) i wypunktowania (`-`, `*`, `•`) usuwa się z tekstu.

## 6. API

Zgodnie z §7.3 pierwszego specu, rozszerzone o `text`. Wszystkie wymagają sesji i CSRF jak reszta API; żaden nie zapisuje przepisu.

- `POST /api/v1/recipes/import/url` z `{ "url": "…" }` zwraca `Draft` (JSON, pola camelCase).
- `POST /api/v1/recipes/import/text` z `{ "text": "…" }` zwraca `Draft`. Limit tekstu 100 KB.
- Błędy: `422 invalid_input` (zły URL, pusty tekst), `422 import_blocked` (adres zablokowany przez SSRF), `422 import_unparsable` (nic nie wyciągnięto; komunikat sugeruje wklejenie tekstu), `504 import_timeout`.
- Zmiany w `api/openapi.yaml`; `TestRoutesMatchOpenAPI` musi przechodzić.
- Limit częstotliwości importu z URL na użytkownika (np. 10/min), żeby serwer nie służył do skanowania sieci.

## 7. Interfejs

**Przepisy → Importuj**: dwie zakładki (Link, Tekst), pole i przycisk "Analizuj". Wynik w podglądzie:

- tytuł, porcje, źródło, tagi (edytowalne);
- tabela składników: ilość, jednostka, nazwa, notatka; wiersze `guess` podświetlone łagodnie, `unparsed` wyraźnie i z oryginalnym tekstem; można poprawić, usunąć, dodać wiersz;
- lista kroków (edytowalna, zmiana kolejności);
- ostrzeżenia nad formularzem;
- "Zapisz" wywołuje istniejące `POST /recipes`;
- duplikaty (ten sam `sourceUrl` lub tytuł): pytanie "zaktualizować czy dodać nowy" przed zapisem.

Na telefonie podgląd jest jedną kolumną; zapis jest przyklejony do dołu ekranu. Przycisk "Importuj" w `RecipesView` obok "Nowy przepis".

## 8. Bezpieczeństwo

- SSRF jak w §5; testowane tablicą adresów (127.0.0.1, ::1, 10/8, 172.16/12, 192.168/16, 169.254/16, adresy tailnetu 100.64/10, IPv4-mapped IPv6) oraz przekierowaniem na adres prywatny i rebindingiem DNS (adres sprawdzany po rozwiązaniu, połączenie do sprawdzonego IP).
- Wynik parsowania to dane: tekst z `Draft` jest przy wyświetlaniu zawsze escapowany; nigdy `v-html`.
- Limity rozmiaru wejścia, liczby linii i długości pól chronią przed nadużyciem.
- Dla samego `Parser` nie ma efektów ubocznych poza ruchem sieciowym `fetch`.

## 9. Testy

- **Linia składnika:** test tablicowy, min. 50 prawdziwych polskich linii (ilości, ułamki, zakresy, brak ilości, jednostki w odmianie, notatki, nawiasy, nietypowe).
- **JSON-LD / microdata:** fixtury zapisanych stron (w tym `@graph`, `HowToSection`, `recipeYield` jako tablica).
- **Tekst:** próbki z nagłówkami, bez nagłówków, z numeracją i wypunktowaniem, PL i EN.
- **Fetch:** tablica SSRF, przekierowania, rebinding, limit rozmiaru i czasu (testowy serwer `httptest`).
- **HTTP:** import nie zapisuje; kody błędów; izolacja sesji i CSRF.
- **Frontend (Vitest):** podgląd renderuje `guess`/`unparsed`, edycja wierszy, zapis składa poprawne `RecipeInput`.
- Zielone: `go test ./...`, `vitest`, `vue-tsc`.

## 10. Kolejność prac

1. `ingredient.go` + tabela testów (+ aliasy w `units`).
2. `text.go`.
3. `fetch.go` z SSRF.
4. `jsonld.go`, `microdata.go`.
5. Endpointy, OpenAPI, limit częstotliwości.
6. UI importu z podglądem; duplikaty.
7. `docs/import.md` (opis działania i ograniczeń), wpis w `CHANGELOG.md`.

## 11. Otwarte kwestie

- Czy wykrywanie duplikatów wchodzi do pierwszej wersji, czy kolejnej (domyślnie: pierwsza, bo dotyczy zapisu z podglądu).
- Lematyzacja nazw ("jajka" → "jajko") celowo pominięta; scalanie na liście zakupów korzysta z `units.NormalizeName`.
