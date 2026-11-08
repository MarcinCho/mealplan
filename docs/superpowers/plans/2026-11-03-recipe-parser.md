# Parser przepisów (import z linku i tekstu) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Użytkownik wkleja link lub tekst przepisu, aplikacja zamienia go na szkic (tytuł, porcje, składniki z ilością i jednostką, kroki), który użytkownik poprawia w edytorze i zapisuje.

**Architecture:** Nowy pakiet `internal/importer` daje wspólny wynik `Draft` z trzech źródeł: parser linii składnika (rdzeń), parser tekstu i parser stron (JSON-LD, microdata, tekst strony) za SSRF-bezpiecznym `Fetcher`. Dwa endpointy `POST /recipes/import/{url,text}` zwracają szkic i niczego nie zapisują. Frontend wczytuje szkic do istniejącego `RecipeEditView` (podświetla wiersze `guess`/`unparsed`), a zapis idzie istniejącym `POST/PUT /recipes`.

**Tech Stack:** Go 1.26 (`net/http`, `golang.org/x/net/html` + `html/charset` jako nowa zależność), Vue 3 + Vuetify 3 + Pinia + Vitest.

**Spec:** `docs/superpowers/specs/2026-11-03-recipe-parser-design.md` (rozszerza §7.3 `2026-11-01-mealplan-design.md`).

## Global Constraints

- Gałąź: `feat/recipe-import`, utworzona z `docs/recipe-parser-spec` (ta bazuje na `feat/today-screen`). Commity z tożsamością noreply (`MarcinCho@users.noreply.github.com`, już w konfiguracji repo). Każdy commit kończy się linią `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`. Nie pushować bez prośby użytkownika.
- Host nie ma Go ani Node. Go: `.superpowers/bin/dkgo go …` (w katalogu `/home/silly/mealplan`). Node: `.superpowers/bin/dknode npm …` (działa w `web/`). Po każdym buildzie frontendu: `touch web/dist/.gitkeep`.
- Limity wejścia zgodne z `internal/recipe`: tytuł 1–200 znaków, porcje 1–100, składniki ≤200 (nazwa 1–200 znaków), kroki ≤100 po ≤2000 znaków, tagi ≤20 po ≤40, `sourceUrl` http(s) do 2000 znaków, ilość ≤1e6.
- Tekst dla użytkownika po polsku; kod, komentarze i identyfikatory po angielsku. Komentarze oszczędne, tylko tam, gdzie powód nie jest oczywisty.
- Odpowiedzi błędów w formacie `{"code","message"}`; `TestRoutesMatchOpenAPI` musi przechodzić po każdej zmianie tras.
- SSRF: tylko http/https, porty domyślne (80/443), blokada adresów loopback, prywatnych, link-local, multicast, unspecified, `0.0.0.0/8` i `100.64.0.0/10` (Tailscale), sprawdzana w `net.Dialer.Control` po rozwiązaniu DNS (dotyczy też przekierowań), max 5 przekierowań, limit 2 MB i 10 s, brak proxy z otoczenia. Treść błędów dialu nigdy nie trafia do odpowiedzi.
- Wynik parsowania to dane: w UI zawsze interpolacja `{{ }}`, nigdy `v-html`.
- Pole `note` składnika służy tylko do podglądu; model przepisu go nie przechowuje, więc znika przy zapisie.

## Review Focus

Wejścia i warunki, które spec implikuje, a które łatwo przeoczyć (każdy ma test w zadaniu wskazanym w nawiasie):

1. Przecinek dziesiętny nie jest separatorem notatki: `1,5 kg ziemniaków` (Task 2).
2. Strona w kodowaniu windows-1250/ISO-8859-2 (częste na polskich serwisach) musi dać poprawne polskie znaki (Task 4).
3. Przekierowanie z publicznego adresu na prywatny i rebinding DNS są blokowane tak samo jak adres bezpośredni (Task 4).
4. Pole składnika `Name` puste lub linia będąca całym zdaniem nie może zniknąć: wiersz `unparsed` zachowuje `raw` (Task 2, Task 7).
5. JSON-LD z `@graph`, tablicą typów, `recipeYield` jako tablicą i `HowToSection` (Task 5).
6. Przepis z linku bez danych strukturalnych i bez nagłówków sekcji nie może zwrócić śmieciowego "przepisu" z menu strony (Task 5).
7. Zapis szkicu nie może wysłać do API pól spoza schematu (`raw`, `note`, `confidence`), bo serwer odrzuca nieznane pola (Task 7).

---

## File Structure

Backend:
- Create `internal/importer/draft.go`: typy, błędy, limity, `finish`.
- Create `internal/importer/ingredient.go`: `ParseIngredient`.
- Create `internal/importer/text.go`: `ParseText`.
- Create `internal/importer/fetch.go`: `Fetcher`, `blockedAddr`.
- Create `internal/importer/html.go`: `ParseHTML` (JSON-LD, microdata, tekst strony).
- Create `internal/importer/service.go`: `Service.Import`.
- Modify `internal/units/units.go`: więcej aliasów jednostek.
- Create `internal/httpapi/importer.go` (handlery, `draftJSON`, `limiter`); modify `server.go`, `errors.go`.
- Modify `api/openapi.yaml`, `cmd/mealplan/main.go`.
- Tests obok plików: `*_test.go`, plus `internal/importer/testdata/*.html`.

Frontend:
- Modify `web/src/api/types.ts`, `web/src/stores/recipes.ts`, `web/src/messages.ts`, `web/src/router.ts`, `web/src/views/RecipesView.vue`, `web/src/views/RecipeEditView.vue`.
- Create `web/src/importDraft.ts` (+ test), `web/src/stores/import.ts` (+ test), `web/src/views/ImportView.vue`.

Docs: `docs/import.md`, `docs/api.md`, `docs/security.md`, `CHANGELOG.md`, poprawki w spec.

---

### Task 1: Aliasy jednostek w `units`

**Files:**
- Modify: `internal/units/units.go` (mapa `aliases`)
- Test: `internal/units/units_test.go`

**Interfaces:**
- Consumes: istniejące `units.CanonUnit(string) string`.
- Produces: `CanonUnit` sprowadza polskie odmiany i skróty do jednej formy; Task 2 polega na tych formach kanonicznych: `g kg ml l szt łyżka łyżeczka szklanka ząbek plaster szczypta opakowanie puszka pęczek garść kostka główka kromka`.

- [ ] **Step 1: Write the failing test**

Dopisz do `internal/units/units_test.go`:

```go
func TestCanonUnitPolishForms(t *testing.T) {
	cases := map[string]string{
		"łyżki": "łyżka", "łyżek": "łyżka", "Łyżkę": "łyżka", "łyż": "łyżka",
		"łyżeczki": "łyżeczka", "łyżeczek": "łyżeczka",
		"szklanki": "szklanka", "szklanek": "szklanka", "szkl": "szklanka",
		"ząbki": "ząbek", "ząbków": "ząbek",
		"plasterki": "plaster", "plasterków": "plaster",
		"szczypty": "szczypta", "szczypta": "szczypta",
		"opakowania": "opakowanie", "op": "opakowanie",
		"puszki": "puszka", "puszek": "puszka",
		"pęczki": "pęczek", "garści": "garść", "kostki": "kostka",
		"główki": "główka", "kromki": "kromka",
		"litra": "l", "litrów": "l", "kilo": "kg", "gr": "g", "ml": "ml", "mililitrów": "ml",
	}
	for in, want := range cases {
		if got := units.CanonUnit(in); got != want {
			t.Errorf("CanonUnit(%q)=%q want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `.superpowers/bin/dkgo go test ./internal/units/ -run TestCanonUnitPolishForms`
Expected: FAIL (np. `CanonUnit("łyżki")="łyżki" want "łyżka"`).

- [ ] **Step 3: Write minimal implementation**

W `internal/units/units.go` zastąp mapę `aliases`:

```go
var aliases = map[string]string{
	"szt": "szt", "szt.": "szt", "sztuka": "szt", "sztuki": "szt", "sztuk": "szt",
	"gram": "g", "gramy": "g", "gramów": "g", "gr": "g",
	"kilogram": "kg", "kilogramy": "kg", "kilo": "kg",
	"litr": "l", "litry": "l", "litra": "l", "litrów": "l",
	"mililitr": "ml", "mililitry": "ml", "mililitrów": "ml",
	"łyżki": "łyżka", "łyżek": "łyżka", "łyżkę": "łyżka", "łyż": "łyżka",
	"łyżeczki": "łyżeczka", "łyżeczek": "łyżeczka", "łyżeczkę": "łyżeczka",
	"szklanki": "szklanka", "szklanek": "szklanka", "szklankę": "szklanka", "szkl": "szklanka",
	"ząbki": "ząbek", "ząbków": "ząbek",
	"plasterki": "plaster", "plasterków": "plaster",
	"szczypty": "szczypta", "szczyptę": "szczypta",
	"opakowania": "opakowanie", "op": "opakowanie", "op.": "opakowanie",
	"puszki": "puszka", "puszek": "puszka",
	"pęczki": "pęczek", "pęczków": "pęczek",
	"garści": "garść",
	"kostki": "kostka",
	"główki": "główka",
	"kromki": "kromka", "kromek": "kromka",
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `.superpowers/bin/dkgo go test ./internal/units/ ./internal/shopping/`
Expected: PASS (lista zakupów korzysta z `CanonUnit`, więc sprawdzamy też ją).

- [ ] **Step 5: Commit**

```bash
git checkout -b feat/recipe-import
git add internal/units
git commit -m "feat(units): canonical Polish unit forms

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Typy szkicu i parser linii składnika

**Files:**
- Create: `internal/importer/draft.go`
- Create: `internal/importer/ingredient.go`
- Test: `internal/importer/ingredient_test.go`, `internal/importer/draft_test.go`

**Interfaces:**
- Consumes: `units.CanonUnit`, `domain.ErrInvalid`.
- Produces (używane przez Task 3–6):
  - stałe `KindURL="url"`, `KindText="text"`, `ConfOK="ok"`, `ConfGuess="guess"`, `ConfUnparsed="unparsed"`, `MaxTextBytes = 100 << 10`
  - błędy `ErrBlocked`, `ErrTimeout`, `ErrFetch`, `ErrUnparsable`
  - `type Source struct{ Kind, URL, Text string }`
  - `type IngredientLine struct{ Raw, Name string; Quantity *float64; Unit, Note, Confidence string }`
  - `type Draft struct{ Title string; Servings int; SourceURL string; Tags []string; Ingredients []IngredientLine; Steps []string; Warnings []string }`
  - `func (d *Draft) warn(format string, a ...any)`, `func (d *Draft) finish()`
  - `func invalid(format string, a ...any) error` (owija `domain.ErrInvalid`)
  - `func ParseIngredient(raw string) IngredientLine`

- [ ] **Step 1: Write the failing tests**

`internal/importer/ingredient_test.go` (tabela to specyfikacja: gdy wiersz zawodzi, popraw kod; wiersz zmieniaj tylko, jeśli przeczy §4 specu):

```go
package importer

import "testing"

func f(v float64) *float64 { return &v }

func TestParseIngredient(t *testing.T) {
	cases := []struct {
		raw    string
		name   string
		qty    *float64
		unit   string
		note   string
		conf   string
	}{
		{"200 g mąki", "mąki", f(200), "g", "", ConfOK},
		{"1,5 kg ziemniaków", "ziemniaków", f(1.5), "kg", "", ConfOK},
		{"1.5 łyżeczki soli", "soli", f(1.5), "łyżeczka", "", ConfOK},
		{"0,5 kg mięsa mielonego", "mięsa mielonego", f(0.5), "kg", "", ConfOK},
		{"1/2 szklanki mleka", "mleka", f(0.5), "szklanka", "", ConfOK},
		{"3/4 szklanki cukru", "cukru", f(0.75), "szklanka", "", ConfOK},
		{"1 ½ szklanki mąki", "mąki", f(1.5), "szklanka", "", ConfOK},
		{"1 1/2 szklanki mąki", "mąki", f(1.5), "szklanka", "", ConfOK},
		{"½ łyżeczki soli", "soli", f(0.5), "łyżeczka", "", ConfOK},
		{"¾ szklanki cukru", "cukru", f(0.75), "szklanka", "", ConfOK},
		{"półtora szklanki mąki", "mąki", f(1.5), "szklanka", "", ConfOK},
		{"pół litra mleka", "mleka", f(0.5), "l", "", ConfOK},
		{"ćwierć łyżeczki soli", "soli", f(0.25), "łyżeczka", "", ConfOK},
		{"2-3 ząbki czosnku", "czosnku", f(3), "ząbek", "", ConfGuess},
		{"1-2 łyżki oleju", "oleju", f(2), "łyżka", "", ConfGuess},
		{"2–3 jabłka", "jabłka", f(3), "szt", "", ConfGuess},
		{"2 do 3 łyżek oleju", "oleju", f(3), "łyżka", "", ConfGuess},
		{"szczypta soli", "soli", nil, "szczypta", "", ConfOK},
		{"garść orzechów włoskich", "orzechów włoskich", nil, "garść", "", ConfOK},
		{"sól", "sól", nil, "", "", ConfOK},
		{"Sól", "Sól", nil, "", "", ConfOK},
		{"sól do smaku", "sól", nil, "", "do smaku", ConfOK},
		{"pieprz, do smaku", "pieprz", nil, "", "do smaku", ConfOK},
		{"3 jajka", "jajka", f(3), "szt", "", ConfGuess},
		{"4 ziemniaki średnie", "ziemniaki średnie", f(4), "szt", "", ConfGuess},
		{"3 szt. jajek", "jajek", f(3), "szt", "", ConfOK},
		{"250g masła", "masła", f(250), "g", "", ConfOK},
		{"2łyżki cukru", "cukru", f(2), "łyżka", "", ConfOK},
		{"1 cebula, posiekana", "cebula", f(1), "szt", "posiekana", ConfGuess},
		{"100 g masła, rozpuszczonego", "masła", f(100), "g", "rozpuszczonego", ConfOK},
		{"1 puszka pomidorów (400 g)", "pomidorów", f(1), "puszka", "400 g", ConfOK},
		{"1 szklanka (250 ml) mleka", "mleka", f(1), "szklanka", "250 ml", ConfOK},
		{"1 opakowanie drożdży (7 g)", "drożdży", f(1), "opakowanie", "7 g", ConfOK},
		{"500 ml mleka (2%)", "mleka", f(500), "ml", "2%", ConfOK},
		{"200 g cukru do posypania", "cukru", f(200), "g", "do posypania", ConfOK},
		{"2 łyżki oliwy z oliwek", "oliwy z oliwek", f(2), "łyżka", "", ConfOK},
		{"1 łyżeczka proszku do pieczenia", "proszku do pieczenia", f(1), "łyżeczka", "", ConfOK},
		{"- 400 g makaronu spaghetti", "makaronu spaghetti", f(400), "g", "", ConfOK},
		{"• 100 ml śmietanki 30%", "śmietanki 30%", f(100), "ml", "", ConfOK},
		{"* 1 l bulionu", "bulionu", f(1), "l", "", ConfOK},
		{"5 dag sera żółtego", "sera żółtego", f(50), "g", "", ConfOK},
		{"2 ząbki czosnku", "czosnku", f(2), "ząbek", "", ConfOK},
		{"3 plasterki boczku", "boczku", f(3), "plaster", "", ConfOK},
		{"1 kostka masła", "masła", f(1), "kostka", "", ConfOK},
		{"1 główka kapusty", "kapusty", f(1), "główka", "", ConfOK},
		{"2 kromki chleba", "chleba", f(2), "kromka", "", ConfOK},
		{"1 pęczek pietruszki", "pietruszki", f(1), "pęczek", "", ConfOK},
		{"Mąka pszenna - 250 g", "Mąka pszenna", f(250), "g", "", ConfGuess},
		{"Cukier – 100 g", "Cukier", f(100), "g", "", ConfGuess},
		{"Jajka: 3", "Jajka", f(3), "szt", "", ConfGuess},
		{"12", "", nil, "", "", ConfUnparsed},
		{"200 g", "", nil, "", "", ConfUnparsed},
		{"", "", nil, "", "", ConfUnparsed},
		{"   ", "", nil, "", "", ConfUnparsed},
		{"Wymieszaj wszystko dokładnie i odstaw na godzinę do lodówki, aż ciasto wyrośnie.", "", nil, "", "", ConfUnparsed},
		{"0 g mąki", "", nil, "", "", ConfUnparsed},
	}
	for _, c := range cases {
		got := ParseIngredient(c.raw)
		if got.Raw != c.raw {
			t.Errorf("%q: raw=%q", c.raw, got.Raw)
		}
		if got.Name != c.name || got.Unit != c.unit || got.Note != c.note || got.Confidence != c.conf {
			t.Errorf("%q: got name=%q unit=%q note=%q conf=%q; want name=%q unit=%q note=%q conf=%q",
				c.raw, got.Name, got.Unit, got.Note, got.Confidence, c.name, c.unit, c.note, c.conf)
		}
		switch {
		case (got.Quantity == nil) != (c.qty == nil):
			t.Errorf("%q: quantity=%v want %v", c.raw, got.Quantity, c.qty)
		case got.Quantity != nil && (*got.Quantity-*c.qty > 1e-9 || *c.qty-*got.Quantity > 1e-9):
			t.Errorf("%q: quantity=%v want %v", c.raw, *got.Quantity, *c.qty)
		}
	}
}

func TestParseIngredientNeverLosesTheLine(t *testing.T) {
	for _, raw := range []string{"asdf ;;; ((", "1 (", "/", "½", "100000000 g mąki", "1 kg mąki"} {
		got := ParseIngredient(raw)
		if got.Raw != raw {
			t.Errorf("%q: raw lost: %q", raw, got.Raw)
		}
	}
	if got := ParseIngredient("1 kg mąki"); got.Name != "mąki" || got.Unit != "kg" {
		t.Errorf("non-breaking spaces: %+v", got)
	}
}
```

`internal/importer/draft_test.go`:

```go
package importer

import (
	"errors"
	"strings"
	"testing"

	"mealplan/internal/domain"
)

func TestFinishEnforcesRecipeLimits(t *testing.T) {
	d := Draft{
		Title:     strings.Repeat("ż", 250),
		Servings:  500,
		SourceURL: "ftp://example.com/r",
		Tags:      []string{" Obiad ", "obiad", "", strings.Repeat("a", 41), "Zupa"},
		Steps:     []string{"  ", strings.Repeat("k", 2500), "ok"},
	}
	for i := 0; i < 210; i++ {
		d.Ingredients = append(d.Ingredients, IngredientLine{Raw: "x", Name: "x"})
	}
	d.finish()
	if n := len([]rune(d.Title)); n != 200 {
		t.Errorf("title runes=%d", n)
	}
	if d.Servings != 0 || d.SourceURL != "" {
		t.Errorf("servings=%d url=%q", d.Servings, d.SourceURL)
	}
	if got := strings.Join(d.Tags, ","); got != "obiad,zupa" {
		t.Errorf("tags=%q", got)
	}
	if len(d.Steps) != 2 || len([]rune(d.Steps[0])) != 2000 {
		t.Errorf("steps=%d first=%d", len(d.Steps), len([]rune(d.Steps[0])))
	}
	if len(d.Ingredients) != 200 {
		t.Errorf("ingredients=%d", len(d.Ingredients))
	}
	if len(d.Warnings) < 4 {
		t.Errorf("expected warnings for every cut, got %v", d.Warnings)
	}
	if d.Tags == nil || d.Steps == nil || d.Ingredients == nil || d.Warnings == nil {
		t.Error("slices must be non-nil for JSON")
	}
}

func TestFinishWarnsAboutMissingParts(t *testing.T) {
	d := Draft{Title: "Zupa"}
	d.finish()
	joined := strings.Join(d.Warnings, "|")
	for _, want := range []string{"porcji", "składników", "kroków"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing warning about %q: %v", want, d.Warnings)
		}
	}
}

func TestInvalidWrapsDomainError(t *testing.T) {
	if err := invalid("bad %s", "x"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("not wrapped: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `.superpowers/bin/dkgo go test ./internal/importer/`
Expected: FAIL (build error: pakiet/symbole nie istnieją).

- [ ] **Step 3: Write the implementation**

`internal/importer/draft.go`:

```go
// Package importer turns a link or pasted text into a recipe Draft that the user reviews before saving.
package importer

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"mealplan/internal/domain"
)

const (
	KindURL  = "url"
	KindText = "text"

	ConfOK       = "ok"
	ConfGuess    = "guess"
	ConfUnparsed = "unparsed"

	MaxTextBytes = 100 << 10

	maxTitle       = 200
	maxServings    = 100
	maxIngredients = 200
	maxSteps       = 100
	maxStepLen     = 2000
	maxTags        = 20
	maxTagLen      = 40
	maxURL         = 2000
	maxQuantity    = 1e6
)

var (
	ErrBlocked    = errors.New("importer: address not allowed")
	ErrTimeout    = errors.New("importer: timed out")
	ErrFetch      = errors.New("importer: page could not be fetched")
	ErrUnparsable = errors.New("importer: no recipe found")
)

type Source struct {
	Kind string
	URL  string
	Text string
}

type IngredientLine struct {
	Raw        string
	Name       string
	Quantity   *float64
	Unit       string
	Note       string
	Confidence string
}

type Draft struct {
	Title       string
	Servings    int // 0 means unknown
	SourceURL   string
	Tags        []string
	Ingredients []IngredientLine
	Steps       []string
	Warnings    []string
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalid, fmt.Sprintf(format, a...))
}

func (d *Draft) warn(format string, a ...any) {
	d.Warnings = append(d.Warnings, fmt.Sprintf(format, a...))
}

func cut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// finish enforces the recipe limits so a reviewed draft can always be saved, and reports anything it had to cut.
func (d *Draft) finish() {
	d.Title = strings.TrimSpace(d.Title)
	if utf8.RuneCountInString(d.Title) > maxTitle {
		d.Title = cut(d.Title, maxTitle)
		d.warn("Tytuł został skrócony do %d znaków.", maxTitle)
	}
	if d.Servings < 0 || d.Servings > maxServings {
		d.Servings = 0
	}
	if d.Servings == 0 {
		d.warn("Nie znaleziono liczby porcji, ustaw ją ręcznie.")
	}
	if d.SourceURL = strings.TrimSpace(d.SourceURL); len(d.SourceURL) > maxURL ||
		!(strings.HasPrefix(d.SourceURL, "http://") || strings.HasPrefix(d.SourceURL, "https://")) {
		d.SourceURL = ""
	}

	if len(d.Ingredients) == 0 {
		d.warn("Nie znaleziono składników.")
	}
	if len(d.Ingredients) > maxIngredients {
		d.Ingredients = d.Ingredients[:maxIngredients]
		d.warn("Pominięto składniki ponad limit %d.", maxIngredients)
	}

	steps := make([]string, 0, len(d.Steps))
	for _, s := range d.Steps {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		if utf8.RuneCountInString(s) > maxStepLen {
			s = cut(s, maxStepLen)
			d.warn("Jeden z kroków został skrócony do %d znaków.", maxStepLen)
		}
		steps = append(steps, s)
	}
	if len(steps) == 0 {
		d.warn("Nie znaleziono kroków.")
	}
	if len(steps) > maxSteps {
		steps = steps[:maxSteps]
		d.warn("Pominięto kroki ponad limit %d.", maxSteps)
	}
	d.Steps = steps

	seen := map[string]bool{}
	tags := make([]string, 0, len(d.Tags))
	for _, tg := range d.Tags {
		tg = strings.ToLower(strings.TrimSpace(tg))
		if tg == "" || utf8.RuneCountInString(tg) > maxTagLen || seen[tg] || len(tags) == maxTags {
			continue
		}
		seen[tg] = true
		tags = append(tags, tg)
	}
	d.Tags = tags

	if d.Ingredients == nil {
		d.Ingredients = []IngredientLine{}
	}
	if d.Warnings == nil {
		d.Warnings = []string{}
	}
}
```

`internal/importer/ingredient.go`:

```go
package importer

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"mealplan/internal/units"
)

var knownUnits = map[string]bool{
	"g": true, "kg": true, "ml": true, "l": true, "szt": true,
	"łyżka": true, "łyżeczka": true, "szklanka": true, "ząbek": true, "plaster": true,
	"szczypta": true, "opakowanie": true, "puszka": true, "pęczek": true, "garść": true,
	"kostka": true, "główka": true, "kromka": true,
}

// Units that make no sense without a number ("g soli") are excluded from number-less lines.
var metricUnits = map[string]bool{"g": true, "kg": true, "ml": true, "l": true, "szt": true}

var unicodeFractions = map[rune]float64{'½': 0.5, '¼': 0.25, '¾': 0.75, '⅓': 1.0 / 3, '⅔': 2.0 / 3, '⅛': 0.125}

var noteSuffixes = []string{" do smaku", " do posypania", " do podania", " do smażenia", " do dekoracji", " do wyboru", " opcjonalnie", " ewentualnie"}

var (
	plainNumber = regexp.MustCompile(`^\d+(?:[.,]\d+)?$`)
	parens      = regexp.MustCompile(`\(([^()]*)\)`)
	glued       = regexp.MustCompile(`^([\d.,/½¼¾⅓⅔⅛\-–—]*[\d½¼¾⅓⅔⅛])(\p{L}[\p{L}.]*)$`)
	trailingAmt = regexp.MustCompile(`^(.+?)\s*[-–—:]\s*(\d.*)$`)
)

func isDigits(s string) bool {
	if s == "" || len(s) > 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseFraction(tok string) (float64, bool) {
	if r, size := utf8.DecodeRuneInString(tok); size > 0 && size == len(tok) {
		f, ok := unicodeFractions[r]
		return f, ok
	}
	a, b, ok := strings.Cut(tok, "/")
	if !ok || !isDigits(a) || !isDigits(b) {
		return 0, false
	}
	n, _ := strconv.Atoi(a)
	d, _ := strconv.Atoi(b)
	if d == 0 {
		return 0, false
	}
	return float64(n) / float64(d), true
}

func parseNumber(tok string) (float64, bool) {
	if f, ok := parseFraction(tok); ok {
		return f, true
	}
	// digits followed by a unicode fraction, e.g. "1½"
	if r, size := utf8.DecodeLastRuneInString(tok); size > 0 && size < len(tok) {
		if f, ok := unicodeFractions[r]; ok {
			if base := tok[:len(tok)-size]; plainNumber.MatchString(base) {
				v, err := strconv.ParseFloat(strings.Replace(base, ",", ".", 1), 64)
				return v + f, err == nil
			}
		}
	}
	if plainNumber.MatchString(tok) {
		v, err := strconv.ParseFloat(strings.Replace(tok, ",", ".", 1), 64)
		return v, err == nil
	}
	return 0, false
}

// parseRangeToken reads "2-3" and returns the upper bound.
func parseRangeToken(tok string) (float64, bool) {
	for i, r := range tok {
		if i > 0 && (r == '-' || r == '–' || r == '—') {
			a, ok1 := parseNumber(tok[:i])
			b, ok2 := parseNumber(tok[i+utf8.RuneLen(r):])
			if ok1 && ok2 {
				return math.Max(a, b), true
			}
		}
	}
	return 0, false
}

// parseQuantity reads an amount from the start of tk. It returns the value, how many tokens it used and
// whether the value is a guess (a range collapsed to its upper bound).
func parseQuantity(tk []string) (v float64, n int, guess bool, ok bool) {
	if len(tk) == 0 {
		return 0, 0, false, false
	}
	switch strings.ToLower(tk[0]) {
	case "pół":
		return 0.5, 1, false, true
	case "ćwierć":
		return 0.25, 1, false, true
	case "półtora", "półtorej":
		return 1.5, 1, false, true
	}
	if r, ok := parseRangeToken(tk[0]); ok {
		return r, 1, true, true
	}
	v, ok = parseNumber(tk[0])
	if !ok {
		return 0, 0, false, false
	}
	if len(tk) > 1 && isDigits(tk[0]) {
		if f, ok := parseFraction(tk[1]); ok {
			return v + f, 2, false, true
		}
	}
	if len(tk) > 2 && (tk[1] == "do" || tk[1] == "-" || tk[1] == "–") {
		if w, ok := parseNumber(tk[2]); ok {
			return math.Max(v, w), 3, true, true
		}
	}
	return v, 1, false, true
}

// takeUnit reads a unit from the start of tk. "dag" and "dkg" are converted to grams through mult.
func takeUnit(tk []string) (unit string, mult float64, n int) {
	if len(tk) == 0 {
		return "", 1, 0
	}
	t := strings.ToLower(strings.TrimRight(tk[0], "."))
	if t == "dag" || t == "dkg" {
		return "g", 10, 1
	}
	if c := units.CanonUnit(t); knownUnits[c] {
		return c, 1, 1
	}
	return "", 1, 0
}

func cleanLine(raw string) string {
	s := strings.NewReplacer(" ", " ", " ", " ", " ", " ", "\t", " ").Replace(raw)
	s = strings.TrimLeft(s, "-*•–—· ")
	return strings.TrimRight(strings.TrimSpace(s), " .;,")
}

// splitNote pulls parenthesised parts and text after a comma (not a decimal comma) out of the line.
func splitNote(s string) (main, note string) {
	var notes []string
	s = parens.ReplaceAllStringFunc(s, func(m string) string {
		if inner := strings.TrimSpace(m[1 : len(m)-1]); inner != "" {
			notes = append(notes, inner)
		}
		return " "
	})
	rs := []rune(s)
	for i, r := range rs {
		if r != ',' {
			continue
		}
		decimal := i > 0 && i+1 < len(rs) && rs[i-1] >= '0' && rs[i-1] <= '9' && rs[i+1] >= '0' && rs[i+1] <= '9'
		if decimal {
			continue
		}
		if tail := strings.TrimSpace(string(rs[i+1:])); tail != "" {
			notes = append(notes, tail)
		}
		s = string(rs[:i])
		break
	}
	return strings.Join(strings.Fields(s), " "), strings.Join(notes, ", ")
}

// tokens splits a line on spaces and separates glued amounts such as "250g" or "2łyżki".
func tokens(main string) []string {
	var out []string
	for _, f := range strings.Fields(main) {
		if m := glued.FindStringSubmatch(f); m != nil {
			if _, _, n := takeUnit([]string{m[2]}); n > 0 {
				out = append(out, m[1], m[2])
				continue
			}
		}
		out = append(out, f)
	}
	return out
}

func hasDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// ParseIngredient splits one ingredient line into amount, unit, name and note. It never drops the line:
// anything it cannot make sense of is returned as ConfUnparsed with Raw intact.
func ParseIngredient(raw string) IngredientLine {
	line := IngredientLine{Raw: raw, Confidence: ConfUnparsed}
	s := cleanLine(raw)
	if s == "" || len(strings.Fields(s)) > 10 {
		return line
	}
	main, note := splitNote(s)
	tk := tokens(main)
	if len(tk) == 0 {
		return line
	}

	var (
		qty      *float64
		unit     string
		guess    bool
		consumed bool
		rest     = tk
	)
	if v, n, rng, ok := parseQuantity(tk); ok {
		u, mult, un := takeUnit(tk[n:])
		v *= mult
		qty, unit, rest, guess, consumed = &v, u, tk[n+un:], rng, true
		if unit == "" {
			unit, guess = "szt", true
		}
	} else if u, mult, un := takeUnit(tk); un > 0 && !metricUnits[u] && mult == 1 {
		unit, rest, consumed = u, tk[un:], true
	} else if m := trailingAmt.FindStringSubmatch(main); m != nil {
		tail := tokens(m[2])
		if v, n, _, ok := parseQuantity(tail); ok {
			u, mult, un := takeUnit(tail[n:])
			if n+un == len(tail) {
				v *= mult
				qty, unit, rest, guess, consumed = &v, u, strings.Fields(m[1]), true, true
				if unit == "" {
					unit = "szt"
				}
			}
		}
	}

	name := strings.Trim(strings.Join(rest, " "), " ,;:-–—")
	if consumed {
		if low := strings.ToLower(name); strings.HasPrefix(low, "z ") || strings.HasPrefix(low, "ze ") {
			name = name[strings.IndexByte(name, ' ')+1:]
		}
	}
	low := strings.ToLower(name)
	for _, suf := range noteSuffixes {
		if strings.HasSuffix(low, suf) && len(name) > len(suf) {
			tail := strings.TrimSpace(name[len(name)-len(suf):])
			name = strings.TrimSpace(name[:len(name)-len(suf)])
			if note == "" {
				note = tail
			} else {
				note = tail + ", " + note
			}
			break
		}
	}

	switch {
	case name == "" || utf8.RuneCountInString(name) > maxTitle:
		return line
	case qty != nil && (*qty <= 0 || *qty > maxQuantity || math.IsNaN(*qty)):
		return line
	}
	line.Name, line.Quantity, line.Unit, line.Note = name, qty, unit, note
	switch {
	case guess, qty == nil && unit == "" && hasDigit(name):
		line.Confidence = ConfGuess
	default:
		line.Confidence = ConfOK
	}
	return line
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `.superpowers/bin/dkgo go test ./internal/importer/ ./internal/units/`
Expected: PASS. Przy niezgodności wiersza tabeli napraw `ingredient.go`, nie tabelę (chyba że wiersz przeczy §4 specu; wtedy opisz zmianę w commicie).

- [ ] **Step 5: Commit**

```bash
git add internal/importer
git commit -m "feat(importer): draft types and ingredient line parser

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Parser wklejonego tekstu

**Files:**
- Create: `internal/importer/text.go`
- Test: `internal/importer/text_test.go`

**Interfaces:**
- Consumes: `ParseIngredient`, `Draft` (Task 2).
- Produces: `func ParseText(text string) Draft` (bez `finish`; nie ustawia ostrzeżeń o braku składników/kroków/porcji, robi to `finish`; ustawia ostrzeżenie o pominiętych liniach). Używane przez Task 5 (`html.go`) i Task 6.

- [ ] **Step 1: Write the failing tests**

`internal/importer/text_test.go`:

```go
package importer

import (
	"strings"
	"testing"
)

func names(d Draft) []string {
	var out []string
	for _, i := range d.Ingredients {
		out = append(out, i.Name)
	}
	return out
}

func TestParseTextWithPolishHeadings(t *testing.T) {
	d := ParseText("Placki ziemniaczane\nPorcje: 4\n\nSkładniki:\n- 1 kg ziemniaków\n- 2 jajka\n- sól\n\nPrzygotowanie:\n1. Zetrzyj ziemniaki.\n2. Dodaj jajka i sól.\n3. Smaż na złoto.\n")
	if d.Title != "Placki ziemniaczane" || d.Servings != 4 {
		t.Fatalf("title=%q servings=%d", d.Title, d.Servings)
	}
	if got := strings.Join(names(d), "|"); got != "ziemniaków|jajka|sól" {
		t.Errorf("ingredients=%s", got)
	}
	if len(d.Steps) != 3 || d.Steps[0] != "Zetrzyj ziemniaki." {
		t.Errorf("steps=%q", d.Steps)
	}
}

func TestParseTextEnglishAndMarkdownHeadings(t *testing.T) {
	d := ParseText("# Pancakes\nServes 2\n## Ingredients\n* 200 g flour\n* 2 eggs\n## Method\n1) Mix.\n2) Fry.")
	if d.Title != "Pancakes" || d.Servings != 2 || len(d.Ingredients) != 2 || len(d.Steps) != 2 {
		t.Fatalf("%+v", d)
	}
}

func TestParseTextServingsForms(t *testing.T) {
	for in, want := range map[string]int{
		"Zupa\nPorcje: 6\nSkładniki\n1 l wody":   6,
		"Zupa\n4 porcje\nSkładniki\n1 l wody":    4,
		"Zupa\ndla 3 osób\nSkładniki\n1 l wody":  3,
		"Zupa\nLiczba porcji 8\nSkładniki\n1 l wody": 8,
		"Zupa\nSkładniki\n1 l wody":              0,
	} {
		if got := ParseText(in).Servings; got != want {
			t.Errorf("%q: servings=%d want %d", in, got, want)
		}
	}
}

func TestParseTextWithoutHeadingsClassifiesLines(t *testing.T) {
	d := ParseText("Naleśniki\n250 g mąki\n2 jajka\n500 ml mleka\nWszystkie składniki dokładnie wymieszaj w misce.\nSmaż na rozgrzanej patelni po minucie z każdej strony.\n")
	if d.Title != "Naleśniki" || len(d.Ingredients) != 3 || len(d.Steps) != 2 {
		t.Fatalf("%+v", d)
	}
}

func TestParseTextMergesWrappedStepsWhenNumbered(t *testing.T) {
	d := ParseText("Zupa\nSkładniki\n1 l wody\nPrzygotowanie\n1. Zagotuj wodę\ni dodaj sól.\n2. Gotuj 10 minut.")
	if len(d.Steps) != 2 || d.Steps[0] != "Zagotuj wodę i dodaj sól." {
		t.Fatalf("steps=%q", d.Steps)
	}
}

func TestParseTextWithoutMarkersEveryLineIsAStep(t *testing.T) {
	d := ParseText("Zupa\nSkładniki\n1 l wody\nPrzygotowanie\nZagotuj wodę.\nDodaj sól.")
	if len(d.Steps) != 2 {
		t.Fatalf("steps=%q", d.Steps)
	}
}

func TestParseTextSkipsSubLabelsAndReportsIgnoredLines(t *testing.T) {
	d := ParseText("Ciasto\nTo jest wstęp do przepisu, który nie należy do żadnej sekcji.\nSkładniki\nNa ciasto:\n200 g mąki\nPrzygotowanie\nWymieszaj.")
	if len(d.Ingredients) != 1 || d.Ingredients[0].Name != "mąki" {
		t.Fatalf("ingredients=%+v", d.Ingredients)
	}
	if len(d.Warnings) == 0 || !strings.Contains(d.Warnings[0], "1") {
		t.Errorf("expected a warning about the skipped intro line: %v", d.Warnings)
	}
}

func TestParseTextKeepsUnparsableIngredientLines(t *testing.T) {
	d := ParseText("Zupa\nSkładniki\n200 g mąki\n???\nPrzygotowanie\nGotuj.")
	if len(d.Ingredients) != 2 || d.Ingredients[1].Raw != "???" {
		t.Fatalf("%+v", d.Ingredients)
	}
}

func TestParseTextEmptyAndFirstLineIngredient(t *testing.T) {
	if d := ParseText("  \n\n"); d.Title != "" || len(d.Ingredients) != 0 || len(d.Steps) != 0 {
		t.Errorf("%+v", d)
	}
	if d := ParseText("200 g mąki\n2 jajka"); d.Title != "" || len(d.Ingredients) != 2 {
		t.Errorf("an ingredient line must not become the title: %+v", d)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `.superpowers/bin/dkgo go test ./internal/importer/ -run ParseText`
Expected: FAIL (undefined: ParseText).

- [ ] **Step 3: Write the implementation**

`internal/importer/text.go`:

```go
package importer

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	secIngredients = "ingredients"
	secSteps       = "steps"
)

var (
	ingHeadings  = []string{"składniki", "skladniki", "ingredients", "ingredient list"}
	stepHeadings = []string{"przygotowanie", "sposób przygotowania", "sposób wykonania", "wykonanie", "kroki", "instrukcje", "method", "instructions", "directions", "preparation", "jak zrobić"}

	h1Re         = regexp.MustCompile(`^#\s+(.+)$`)
	stepMarker   = regexp.MustCompile(`(?i)^\s*(?:(?:krok|step)\s*)?\d{1,3}\s*[.):](?:\s+|$)`)
	bulletMarker = regexp.MustCompile(`^\s*[-*•·]\s+`)
	servingsRes  = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(?:liczba\s+|ilość\s+)?(?:porcje|porcji|porcja|serves|servings|yield|makes)\s*[:\-]?\s*(\d{1,3})\b`),
		regexp.MustCompile(`(?i)^(\d{1,3})\s*(?:porcje|porcji|porcja|osób|osoby|osoba|servings|serves)\b`),
		regexp.MustCompile(`(?i)^dla\s+(\d{1,3})\s+os`),
	}
)

func splitLines(text string) []string {
	text = strings.TrimPrefix(text, "﻿")
	text = strings.NewReplacer("\r\n", "\n", "\r", "\n", " ", " ").Replace(text)
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// heading reports whether a line names a section. Longer lines that merely start with a heading word
// ("Przygotowanie farszu zajmuje chwilę") are not headings.
func heading(line string) (string, bool) {
	s := strings.TrimSpace(strings.TrimLeft(line, "#*_ "))
	low := strings.ToLower(strings.TrimRight(s, ":*_ "))
	if low == "" || utf8.RuneCountInString(low) > 60 {
		return "", false
	}
	short := len(strings.Fields(low)) <= 4
	match := func(list []string) bool {
		for _, h := range list {
			if low == h || (short && strings.HasPrefix(low, h+" ")) {
				return true
			}
		}
		return false
	}
	switch {
	case match(ingHeadings):
		return secIngredients, true
	case match(stepHeadings):
		return secSteps, true
	}
	return "", false
}

func servingsOf(line string) (int, bool) {
	for _, re := range servingsRes {
		if m := re.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			return n, n > 0
		}
	}
	return 0, false
}

// isSubLabel matches group labels inside an ingredient list such as "Na ciasto:".
func isSubLabel(l string) bool {
	return strings.HasSuffix(l, ":") && len(strings.Fields(l)) <= 4 && !hasDigit(l)
}

// classify decides what a line is when the text has no section headings. It returns "" for lines to skip.
func classify(l string) string {
	words := len(strings.Fields(l))
	switch {
	case stepMarker.MatchString(l):
		return secSteps
	case bulletMarker.MatchString(l):
		if words <= 8 {
			return secIngredients
		}
		return secSteps
	}
	if p := ParseIngredient(l); p.Quantity != nil && p.Confidence != ConfUnparsed {
		return secIngredients
	}
	if words >= 4 {
		return secSteps
	}
	return ""
}

func buildSteps(raw []string) []string {
	markers := false
	for _, l := range raw {
		if stepMarker.MatchString(l) || bulletMarker.MatchString(l) {
			markers = true
			break
		}
	}
	var out []string
	for _, l := range raw {
		marked := stepMarker.MatchString(l) || bulletMarker.MatchString(l)
		text := strings.TrimSpace(bulletMarker.ReplaceAllString(stepMarker.ReplaceAllString(l, ""), ""))
		switch {
		case text == "":
		case !markers || marked || len(out) == 0:
			out = append(out, text)
		default:
			out[len(out)-1] += " " + text
		}
	}
	return out
}

// ParseText reads a recipe pasted as plain text. Missing parts are left empty; Draft.finish reports them.
func ParseText(text string) Draft {
	var d Draft
	lines := splitLines(text)

	var body []string
	titleFound := false
	for _, l := range lines {
		if m := h1Re.FindStringSubmatch(l); m != nil && !titleFound {
			if _, isSection := heading(l); !isSection {
				d.Title, titleFound = strings.TrimSpace(m[1]), true
				continue
			}
		}
		if n, ok := servingsOf(l); ok && d.Servings == 0 {
			d.Servings = n
			continue
		}
		body = append(body, l)
	}
	if !titleFound && len(body) > 0 {
		first := body[0]
		_, isSection := heading(first)
		p := ParseIngredient(first)
		if !isSection && !bulletMarker.MatchString(first) && !stepMarker.MatchString(first) &&
			p.Quantity == nil && p.Unit == "" && len(strings.Fields(first)) <= 15 {
			d.Title = first
			body = body[1:]
		}
	}

	var ingRaw, stepRaw, loose []string
	cur, headed := "", false
	for _, l := range body {
		if k, ok := heading(l); ok {
			cur, headed = k, true
			continue
		}
		switch cur {
		case secIngredients:
			if !isSubLabel(l) {
				ingRaw = append(ingRaw, l)
			}
		case secSteps:
			stepRaw = append(stepRaw, l)
		default:
			loose = append(loose, l)
		}
	}
	if !headed {
		skipped := 0
		for _, l := range loose {
			switch classify(l) {
			case secIngredients:
				ingRaw = append(ingRaw, l)
			case secSteps:
				stepRaw = append(stepRaw, l)
			default:
				skipped++
			}
		}
		if skipped > 0 {
			d.warn("Pominięto %d krótkich linii, których nie udało się sklasyfikować.", skipped)
		}
	} else if len(loose) > 0 {
		d.warn("Pominięto %d linii spoza sekcji (np. opis przed składnikami).", len(loose))
	}

	for _, l := range ingRaw {
		d.Ingredients = append(d.Ingredients, ParseIngredient(l))
	}
	d.Steps = buildSteps(stepRaw)
	return d
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `.superpowers/bin/dkgo go test ./internal/importer/`
Expected: PASS. Uwaga: `TestParseTextSkipsSubLabelsAndReportsIgnoredLines` oczekuje, że tytułem jest `Ciasto`, a linia wstępu trafia do pominiętych; jeśli klasyfikacja tytułu zachowuje się inaczej, popraw kod `ParseText`, nie test.

- [ ] **Step 5: Commit**

```bash
git add internal/importer
git commit -m "feat(importer): parse pasted recipe text

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Pobieranie stron z ochroną SSRF

**Files:**
- Create: `internal/importer/fetch.go`
- Modify: `go.mod`, `go.sum` (zależność `golang.org/x/net`)
- Test: `internal/importer/fetch_test.go`

**Interfaces:**
- Consumes: błędy z `draft.go`, `invalid`.
- Produces:
  - `type Fetcher struct{ … }`, `func NewFetcher() *Fetcher` (produkcyjny: blokady włączone, tylko porty 80/443, timeout 10 s, 2 MB)
  - `func (f *Fetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error)` zwraca treść w UTF-8; błędy: owinięte `domain.ErrInvalid` (zły adres), `ErrBlocked`, `ErrTimeout`, `ErrFetch`
  - `func blockedAddr(a netip.Addr) bool`
  - `func newFetcher(allow func(netip.Addr) bool, timeout time.Duration, anyPort bool) *Fetcher` (pakietowy, dla testów)

- [ ] **Step 1: Add the dependency**

Run: `.superpowers/bin/dkgo go get golang.org/x/net@latest`
Expected: `go.mod` zyskuje `golang.org/x/net`. (Zostanie użyta w Task 4 i 5; `go mod tidy` robimy na końcu Task 5.)

- [ ] **Step 2: Write the failing tests**

`internal/importer/fetch_test.go`:

```go
package importer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"mealplan/internal/domain"
)

func allowAll(netip.Addr) bool { return true }

func newTestFetcher() *Fetcher { return newFetcher(allowAll, 2*time.Second, true) }

func TestBlockedAddr(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "::1", "10.0.0.5", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "100.64.0.1", "100.100.100.100", "0.0.0.0", "0.1.2.3",
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "fe80::1", "fc00::1", "fd12:3456::1", "ff02::1", "224.0.0.1",
	}
	for _, s := range blocked {
		if !blockedAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s must be blocked", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "93.184.216.34", "172.32.0.1", "100.128.0.1", "2606:4700::1111"} {
		if blockedAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s must be allowed", s)
		}
	}
}

func TestFetchRejectsBadURLs(t *testing.T) {
	f := NewFetcher()
	for _, u := range []string{"", "ftp://example.com/x", "file:///etc/passwd", "javascript:alert(1)", "http://", "https://user:pw@example.com/", "http://example.com:22/", "http://example.com:8080/", "not a url", "https://" + strings.Repeat("a", 2100) + ".com"} {
		if _, err := f.Fetch(context.Background(), u); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%q: expected ErrInvalid, got %v", u, err)
		}
	}
}

func TestFetchBlocksPrivateTargetsByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer srv.Close()
	f := newFetcher(func(a netip.Addr) bool { return !blockedAddr(a) }, 2*time.Second, true)
	if _, err := f.Fetch(context.Background(), srv.URL); !errors.Is(err, ErrBlocked) {
		t.Fatalf("loopback server must be blocked, got %v", err)
	}
	for _, u := range []string{"http://127.0.0.1/", "http://[::1]/", "http://169.254.169.254/latest/meta-data/", "http://10.0.0.1/", "http://100.64.0.1/", "http://localhost/"} {
		if _, err := NewFetcher().Fetch(context.Background(), u); !errors.Is(err, ErrBlocked) {
			t.Errorf("%s: expected ErrBlocked, got %v", u, err)
		}
	}
}

func TestFetchBlocksRedirectToPrivateAddress(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("internal")) }))
	defer target.Close()
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer front.Close()
	dials := 0
	f := newFetcher(func(netip.Addr) bool { dials++; return dials == 1 }, 2*time.Second, true)
	if _, err := f.Fetch(context.Background(), front.URL); !errors.Is(err, ErrBlocked) {
		t.Fatalf("redirect must be re-checked, got %v", err)
	}
}

func TestFetchFollowsAtMostFiveRedirects(t *testing.T) {
	n := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		http.Redirect(w, r, srv.URL+"/next", http.StatusFound)
	}))
	defer srv.Close()
	if _, err := newTestFetcher().Fetch(context.Background(), srv.URL); !errors.Is(err, ErrFetch) {
		t.Fatalf("expected ErrFetch, got %v", err)
	}
	if n > 6 {
		t.Errorf("followed %d requests", n)
	}
}

func TestFetchReadsHTMLAndDecodesLegacyCharsets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=iso-8859-2")
		_, _ = w.Write([]byte("<p>\xb3\xf3d\xbc</p>")) // "łódź" in ISO-8859-2
	}))
	defer srv.Close()
	body, err := newTestFetcher().Fetch(context.Background(), srv.URL)
	if err != nil || !strings.Contains(string(body), "łódź") {
		t.Fatalf("body=%q err=%v", body, err)
	}
}

func TestFetchLimitsAndFailures(t *testing.T) {
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(strings.Repeat("a", 3<<20)))
	}))
	defer big.Close()
	if _, err := newTestFetcher().Fetch(context.Background(), big.URL); !errors.Is(err, ErrFetch) {
		t.Errorf("oversized body: %v", err)
	}

	notFound := httptest.NewServer(http.NotFoundHandler())
	defer notFound.Close()
	if _, err := newTestFetcher().Fetch(context.Background(), notFound.URL); !errors.Is(err, ErrFetch) {
		t.Errorf("404: %v", err)
	}

	pdf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF"))
	}))
	defer pdf.Close()
	if _, err := newTestFetcher().Fetch(context.Background(), pdf.URL); !errors.Is(err, ErrFetch) {
		t.Errorf("pdf: %v", err)
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { time.Sleep(500 * time.Millisecond) }))
	defer slow.Close()
	f := newFetcher(allowAll, 100*time.Millisecond, true)
	if _, err := f.Fetch(context.Background(), slow.URL); !errors.Is(err, ErrTimeout) {
		t.Errorf("slow server: %v", err)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `.superpowers/bin/dkgo go test ./internal/importer/ -run 'Fetch|Blocked'`
Expected: FAIL (undefined: Fetcher, newFetcher, blockedAddr).

- [ ] **Step 4: Write the implementation**

`internal/importer/fetch.go`:

```go
package importer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html/charset"
)

const (
	maxPageBytes = 2 << 20
	maxRedirects = 5
	userAgent    = "Mealplan/1.0 (recipe import)"
)

var (
	cgnat   = netip.MustParsePrefix("100.64.0.0/10") // Tailscale and carrier-grade NAT
	thisNet = netip.MustParsePrefix("0.0.0.0/8")

	errInvalidRedirect = errors.New("importer: redirect to unsupported address")
)

// blockedAddr reports addresses the server must never connect to on a user's behalf.
func blockedAddr(a netip.Addr) bool {
	a = a.Unmap()
	return !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
		a.IsMulticast() || a.IsUnspecified() || cgnat.Contains(a) || thisNet.Contains(a)
}

type Fetcher struct {
	client   *http.Client
	maxBytes int64
	anyPort  bool
}

func NewFetcher() *Fetcher {
	return newFetcher(func(a netip.Addr) bool { return !blockedAddr(a) }, 10*time.Second, false)
}

// newFetcher checks every connection in the dialer after DNS resolution, so redirects and DNS rebinding
// cannot reach an address that allow rejects.
func newFetcher(allow func(netip.Addr) bool, timeout time.Duration, anyPort bool) *Fetcher {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return ErrBlocked
			}
			ip, err := netip.ParseAddr(host)
			if err != nil || !allow(ip) {
				return ErrBlocked
			}
			return nil
		},
	}
	tr := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
		DisableKeepAlives:     true,
	}
	return &Fetcher{
		maxBytes: maxPageBytes,
		anyPort:  anyPort,
		client: &http.Client{
			Transport: tr,
			Timeout:   timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > maxRedirects {
					return ErrFetch
				}
				if err := checkURL(req.URL, anyPort); err != nil {
					return errInvalidRedirect
				}
				return nil
			},
		},
	}
}

func checkURL(u *url.URL, anyPort bool) error {
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return invalid("obsługiwane są tylko adresy http i https")
	case u.Hostname() == "":
		return invalid("adres nie zawiera nazwy serwera")
	case u.User != nil:
		return invalid("adresy z danymi logowania nie są obsługiwane")
	case !anyPort && u.Port() != "" && u.Port() != "80" && u.Port() != "443":
		return invalid("obsługiwane są tylko domyślne porty")
	}
	return nil
}

func fetchErr(err error) error {
	var ne net.Error
	switch {
	case errors.Is(err, ErrBlocked):
		return ErrBlocked
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return ErrTimeout
	}
	// The cause is deliberately dropped: dial errors would tell the caller which internal hosts exist.
	return ErrFetch
}

// Fetch downloads a web page and returns it decoded to UTF-8.
func (f *Fetcher) Fetch(ctx context.Context, raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxURL {
		return nil, invalid("adres musi mieć od 1 do %d znaków", maxURL)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, invalid("adres jest nieprawidłowy")
	}
	if err := checkURL(u, f.anyPort); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, invalid("adres jest nieprawidłowy")
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")
	req.Header.Set("Accept-Language", "pl,en;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, errInvalidRedirect) || errors.Is(err, ErrFetch) {
			return nil, ErrFetch
		}
		return nil, fetchErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, ErrFetch
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "" {
		if mt, _, err := mime.ParseMediaType(ct); err != nil || (mt != "text/html" && mt != "application/xhtml+xml") {
			return nil, ErrFetch
		}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBytes+1))
	if err != nil {
		return nil, fetchErr(err)
	}
	if int64(len(data)) > f.maxBytes {
		return nil, ErrFetch
	}
	r, err := charset.NewReader(bytes.NewReader(data), ct)
	if err != nil {
		return nil, ErrFetch
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, ErrFetch
	}
	return out, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `.superpowers/bin/dkgo gofmt -l internal/importer && .superpowers/bin/dkgo go vet ./internal/importer/ && .superpowers/bin/dkgo go test ./internal/importer/`
Expected: brak plików z `gofmt -l` (jeśli są, uruchom `gofmt -w`), vet czysty, testy PASS. Test `TestFetchBlocksPrivateTargetsByDefault` dla `http://localhost/` zależy od rozwiązania `localhost` do loopbacku; to oczekiwane w Dockerze z `--network host`.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/importer
git commit -m "feat(importer): SSRF-safe page fetcher

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Parser stron (JSON-LD, microdata, tekst strony) i `Service`

**Files:**
- Create: `internal/importer/html.go`
- Create: `internal/importer/service.go`
- Create: `internal/importer/testdata/jsonld_graph.html`, `testdata/jsonld_plain.html`, `testdata/microdata.html`, `testdata/plain_page.html`, `testdata/menu_only.html`
- Test: `internal/importer/html_test.go`, `internal/importer/service_test.go`

**Interfaces:**
- Consumes: `ParseIngredient`, `ParseText`, `Draft.finish`, `Fetcher.Fetch`, `newTestFetcher` (z testów Task 4, ten sam pakiet).
- Produces:
  - `func ParseHTML(body []byte) (Draft, error)` (`ErrUnparsable`, gdy brak przepisu); nie wywołuje `finish`
  - `type Service struct{ fetcher *Fetcher }`, `func New() *Service`
  - `func (s *Service) Import(ctx context.Context, src Source) (Draft, error)` zwraca szkic po `finish`; błędy: `domain.ErrInvalid`, `ErrBlocked`, `ErrTimeout`, `ErrFetch`, `ErrUnparsable`. Interfejs `httpapi.Importer` z Task 6 jest dokładnie tą sygnaturą.

- [ ] **Step 1: Create the fixtures**

`internal/importer/testdata/jsonld_graph.html`:

```html
<!doctype html><html><head><title>Zupa pomidorowa | Kuchnia</title>
<script type="application/ld+json">
{"@context":"https://schema.org","@graph":[
 {"@type":"WebSite","name":"Kuchnia"},
 {"@type":["Recipe","NewsArticle"],"name":"Zupa pomidorowa &amp; ryż",
  "recipeYield":["4","4 porcje"],
  "keywords":"zupa, Obiad, szybkie",
  "recipeCategory":["Dania główne"],
  "recipeIngredient":["1 l bulionu","2 łyżki koncentratu pomidorowego","1/2 szklanki ryżu","sól do smaku"],
  "recipeInstructions":[
    {"@type":"HowToSection","name":"Zupa","itemListElement":[
      {"@type":"HowToStep","text":"Zagotuj bulion."},
      {"@type":"HowToStep","text":"Dodaj <b>koncentrat</b> i ryż."}]},
    {"@type":"HowToStep","text":"Gotuj 15 minut."}]}
]}
</script></head><body><h1>Zupa pomidorowa</h1></body></html>
```

`internal/importer/testdata/jsonld_plain.html`:

```html
<!doctype html><html><head>
<script type="application/ld+json">{"@context":"https://schema.org","@type":"Recipe","name":"Naleśniki",
"recipeYield":8,"recipeIngredient":["250 g mąki","2 jajka","500 ml mleka"],
"recipeInstructions":"Wymieszaj składniki.\nSmaż na patelni.<br>Podawaj z dżemem."}</script>
</head><body></body></html>
```

`internal/importer/testdata/microdata.html`:

```html
<!doctype html><html><body>
<div itemscope itemtype="http://schema.org/Recipe">
  <h1 itemprop="name">Sałatka grecka</h1>
  <meta itemprop="recipeYield" content="2 porcje">
  <ul>
    <li itemprop="recipeIngredient">2 pomidory</li>
    <li itemprop="recipeIngredient">100 g fety</li>
    <li itemprop="recipeIngredient">oliwa</li>
  </ul>
  <div itemprop="recipeInstructions"><p>Pokrój warzywa.</p><p>Dodaj fetę i polej oliwą.</p></div>
</div></body></html>
```

`internal/importer/testdata/plain_page.html`:

```html
<!doctype html><html><head><title>Placki | Blog</title><style>.x{}</style></head><body>
<nav><a href="/">Strona główna</a><a href="/kontakt">Kontakt</a></nav>
<h1>Placki ziemniaczane</h1>
<h2>Składniki</h2>
<ul><li>1 kg ziemniaków</li><li>2 jajka</li><li>sól</li></ul>
<h2>Przygotowanie</h2>
<ol><li>Zetrzyj ziemniaki.</li><li>Dodaj jajka.</li><li>Smaż.</li></ol>
<footer>Kontakt | Regulamin</footer>
<script>var x = "Składniki";</script>
</body></html>
```

`internal/importer/testdata/menu_only.html`:

```html
<!doctype html><html><head><title>Blog</title></head><body>
<nav><a href="/">Strona główna</a><a href="/przepisy">Przepisy</a><a href="/kontakt">Kontakt</a></nav>
<h1>Witamy na naszym blogu</h1>
<p>Piszemy o gotowaniu, podróżach i życiu codziennym od wielu lat.</p>
</body></html>
```

- [ ] **Step 2: Write the failing tests**

`internal/importer/html_test.go`:

```go
package importer

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseHTMLJSONLDGraph(t *testing.T) {
	d, err := ParseHTML(fixture(t, "jsonld_graph.html"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Zupa pomidorowa & ryż" || d.Servings != 4 {
		t.Errorf("title=%q servings=%d", d.Title, d.Servings)
	}
	if got := strings.Join(names(d), "|"); got != "bulionu|koncentratu pomidorowego|ryżu|sól" {
		t.Errorf("ingredients=%s", got)
	}
	want := []string{"Zagotuj bulion.", "Dodaj koncentrat i ryż.", "Gotuj 15 minut."}
	if strings.Join(d.Steps, "|") != strings.Join(want, "|") {
		t.Errorf("steps=%q", d.Steps)
	}
	d.finish()
	if got := strings.Join(d.Tags, ","); got != "zupa,obiad,szybkie,dania główne" {
		t.Errorf("tags=%q", got)
	}
}

func TestParseHTMLJSONLDPlainWithStringInstructions(t *testing.T) {
	d, err := ParseHTML(fixture(t, "jsonld_plain.html"))
	if err != nil || d.Title != "Naleśniki" || d.Servings != 8 || len(d.Ingredients) != 3 {
		t.Fatalf("%+v %v", d, err)
	}
	if len(d.Steps) != 3 || d.Steps[2] != "Podawaj z dżemem." {
		t.Errorf("steps=%q", d.Steps)
	}
}

func TestParseHTMLMicrodata(t *testing.T) {
	d, err := ParseHTML(fixture(t, "microdata.html"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Sałatka grecka" || d.Servings != 2 || len(d.Ingredients) != 3 || len(d.Steps) != 2 {
		t.Fatalf("%+v", d)
	}
	if d.Ingredients[1].Name != "fety" || *d.Ingredients[1].Quantity != 100 {
		t.Errorf("%+v", d.Ingredients[1])
	}
}

func TestParseHTMLFallsBackToPageTextWithAWarning(t *testing.T) {
	d, err := ParseHTML(fixture(t, "plain_page.html"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Placki ziemniaczane" || len(d.Ingredients) != 3 || len(d.Steps) != 3 {
		t.Fatalf("%+v", d)
	}
	if len(d.Warnings) == 0 || !strings.Contains(d.Warnings[0], "strukturalne") {
		t.Errorf("warnings=%v", d.Warnings)
	}
	for _, s := range d.Steps {
		if strings.Contains(s, "Regulamin") || strings.Contains(s, "var x") {
			t.Errorf("page chrome leaked into steps: %q", s)
		}
	}
}

func TestParseHTMLRefusesPagesWithoutARecipe(t *testing.T) {
	for _, name := range []string{"menu_only.html"} {
		if _, err := ParseHTML(fixture(t, name)); !errors.Is(err, ErrUnparsable) {
			t.Errorf("%s: expected ErrUnparsable, got %v", name, err)
		}
	}
	for _, in := range []string{"", "<html></html>", "<<<>>>", `<script type="application/ld+json">{broken</script>`} {
		if _, err := ParseHTML([]byte(in)); !errors.Is(err, ErrUnparsable) {
			t.Errorf("%q: expected ErrUnparsable, got %v", in, err)
		}
	}
}

func TestParseHTMLIgnoresEmptyJSONLDRecipe(t *testing.T) {
	page := `<script type="application/ld+json">{"@type":"Recipe","name":"Puste"}</script>` +
		`<div itemscope itemtype="https://schema.org/Recipe"><span itemprop="name">Z microdanych</span>` +
		`<span itemprop="recipeIngredient">1 jajko</span></div>`
	d, err := ParseHTML([]byte(page))
	if err != nil || d.Title != "Z microdanych" {
		t.Fatalf("%+v %v", d, err)
	}
}
```

`internal/importer/service_test.go`:

```go
package importer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mealplan/internal/domain"
)

func serve(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestImportURLReturnsFinishedDraftWithSourceURL(t *testing.T) {
	srv := serve(t, fixture(t, "jsonld_plain.html"))
	s := &Service{fetcher: newTestFetcher()}
	d, err := s.Import(context.Background(), Source{Kind: KindURL, URL: " " + srv.URL + "/r "})
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Naleśniki" || d.SourceURL != srv.URL+"/r" || len(d.Warnings) != 0 {
		t.Errorf("%+v", d)
	}
}

func TestImportURLPageWithoutRecipeIsUnparsable(t *testing.T) {
	srv := serve(t, fixture(t, "menu_only.html"))
	s := &Service{fetcher: newTestFetcher()}
	if _, err := s.Import(context.Background(), Source{Kind: KindURL, URL: srv.URL}); !errors.Is(err, ErrUnparsable) {
		t.Fatalf("got %v", err)
	}
}

func TestImportTextValidationAndResult(t *testing.T) {
	s := New()
	ctx := context.Background()
	for _, in := range []string{"", "   \n ", strings.Repeat("a", MaxTextBytes+1)} {
		if _, err := s.Import(ctx, Source{Kind: KindText, Text: in}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("len %d: expected ErrInvalid, got %v", len(in), err)
		}
	}
	if _, err := s.Import(ctx, Source{Kind: KindText, Text: "Dzień dobry, jak się masz?"}); !errors.Is(err, ErrUnparsable) {
		t.Errorf("chatter must be unparsable, got %v", err)
	}
	d, err := s.Import(ctx, Source{Kind: KindText, Text: "Zupa\nPorcje: 2\nSkładniki\n1 l wody\nPrzygotowanie\nZagotuj."})
	if err != nil || d.Title != "Zupa" || d.Servings != 2 || d.SourceURL != "" {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestImportRejectsUnknownKind(t *testing.T) {
	if _, err := New().Import(context.Background(), Source{Kind: "ftp"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `.superpowers/bin/dkgo go test ./internal/importer/ -run 'HTML|Import'`
Expected: FAIL (undefined: ParseHTML, Service, New).

- [ ] **Step 4: Write the implementation**

`internal/importer/html.go`:

```go
package importer

import (
	"bytes"
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

var (
	tagRe    = regexp.MustCompile(`<[^>]*>`)
	breakRe  = regexp.MustCompile(`(?i)<\s*(?:br\s*/?|/p|/li|/div)\s*>`)
	firstNum = regexp.MustCompile(`\d+`)
)

var skipTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "nav": true, "footer": true, "header": true,
	"aside": true, "form": true, "svg": true, "template": true, "iframe": true,
}

var blockTags = map[string]bool{
	"p": true, "div": true, "li": true, "br": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true, "tr": true, "ul": true, "ol": true, "section": true, "article": true,
	"table": true, "dt": true, "dd": true,
}

// ParseHTML extracts a recipe from a web page: JSON-LD first, then microdata, then the visible text.
func ParseHTML(body []byte) (Draft, error) {
	doc, err := xhtml.Parse(bytes.NewReader(body))
	if err != nil {
		return Draft{}, ErrUnparsable
	}
	if d, ok := fromJSONLD(doc); ok && usable(d) {
		return d, nil
	}
	if d, ok := fromMicrodata(doc); ok && usable(d) {
		return d, nil
	}
	return fromPageText(doc)
}

func usable(d Draft) bool { return len(d.Ingredients)+len(d.Steps) > 0 }

func walk(n *xhtml.Node, f func(*xhtml.Node) bool) {
	if !f(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, f)
	}
}

func attr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// plain strips markup and entities and collapses whitespace.
func plain(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(tagRe.ReplaceAllString(s, " "))), " ")
}

func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(breakRe.ReplaceAllString(s, "\n"), "\n") {
		if l = plain(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// blockText returns the visible text under n with a newline around block elements.
func blockText(n *xhtml.Node) string {
	var b strings.Builder
	var rec func(*xhtml.Node)
	rec = func(n *xhtml.Node) {
		switch n.Type {
		case xhtml.TextNode:
			b.WriteString(n.Data)
		case xhtml.ElementNode:
			if skipTags[n.Data] {
				return
			}
			block := blockTags[n.Data]
			if block {
				b.WriteByte('\n')
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				rec(c)
			}
			if block {
				b.WriteByte('\n')
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(n)
	return b.String()
}

func yieldOf(s string) int {
	if m := firstNum.FindString(s); m != "" {
		n, _ := strconv.Atoi(m)
		return n
	}
	return 0
}

func ingredientLines(raw []string) []IngredientLine {
	var out []IngredientLine
	for _, r := range raw {
		for _, l := range lines(r) {
			out = append(out, ParseIngredient(l))
		}
	}
	return out
}

// --- JSON-LD ---

func fromJSONLD(doc *xhtml.Node) (Draft, bool) {
	var found map[string]any
	walk(doc, func(n *xhtml.Node) bool {
		if found != nil {
			return false
		}
		if n.Type == xhtml.ElementNode && n.Data == "script" && strings.EqualFold(attr(n, "type"), "application/ld+json") {
			if n.FirstChild != nil {
				var v any
				if json.Unmarshal([]byte(n.FirstChild.Data), &v) == nil {
					found = findRecipe(v)
				}
			}
			return false
		}
		return true
	})
	if found == nil {
		return Draft{}, false
	}
	return draftFromLD(found), true
}

func isRecipeType(t any) bool {
	switch x := t.(type) {
	case string:
		return x == "Recipe" || strings.HasSuffix(x, "/Recipe")
	case []any:
		for _, e := range x {
			if isRecipeType(e) {
				return true
			}
		}
	}
	return false
}

func findRecipe(v any) map[string]any {
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			if r := findRecipe(e); r != nil {
				return r
			}
		}
	case map[string]any:
		if isRecipeType(x["@type"]) {
			return x
		}
		if g, ok := x["@graph"]; ok {
			return findRecipe(g)
		}
	}
	return nil
}

func ldString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		for _, e := range x {
			if s := ldString(e); s != "" {
				return s
			}
		}
	}
	return ""
}

func ldStrings(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		var out []string
		for _, e := range x {
			if s := ldString(e); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func ldYield(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		return yieldOf(x)
	case []any:
		for _, e := range x {
			if n := ldYield(e); n > 0 {
				return n
			}
		}
	}
	return 0
}

func ldInstructions(v any) []string {
	switch x := v.(type) {
	case string:
		return lines(x)
	case []any:
		var out []string
		for _, e := range x {
			out = append(out, ldInstructions(e)...)
		}
		return out
	case map[string]any:
		if list, ok := x["itemListElement"]; ok {
			return ldInstructions(list)
		}
		if t := ldString(x["text"]); t != "" {
			return lines(t)
		}
		return lines(ldString(x["name"]))
	}
	return nil
}

func ldTags(v any) []string {
	var out []string
	for _, s := range ldStrings(v) {
		for _, p := range strings.Split(s, ",") {
			if p = plain(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func draftFromLD(m map[string]any) Draft {
	var d Draft
	d.Title = plain(ldString(m["name"]))
	d.Servings = ldYield(m["recipeYield"])
	raw := ldStrings(m["recipeIngredient"])
	if len(raw) == 0 {
		raw = ldStrings(m["ingredients"])
	}
	d.Ingredients = ingredientLines(raw)
	d.Steps = ldInstructions(m["recipeInstructions"])
	d.Tags = append(ldTags(m["keywords"]), ldTags(m["recipeCategory"])...)
	return d
}

// --- microdata ---

func propValue(n *xhtml.Node) string {
	switch n.Data {
	case "meta":
		return attr(n, "content")
	case "time":
		if v := attr(n, "datetime"); v != "" {
			return v
		}
	}
	return blockText(n)
}

func fromMicrodata(doc *xhtml.Node) (Draft, bool) {
	var scope *xhtml.Node
	walk(doc, func(n *xhtml.Node) bool {
		if scope != nil {
			return false
		}
		if n.Type == xhtml.ElementNode && strings.HasSuffix(strings.TrimRight(attr(n, "itemtype"), "/"), "/Recipe") {
			scope = n
			return false
		}
		return true
	})
	if scope == nil {
		return Draft{}, false
	}
	props := map[string][]string{}
	var collect func(*xhtml.Node)
	collect = func(n *xhtml.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != xhtml.ElementNode {
				continue
			}
			if p := attr(c, "itemprop"); p != "" {
				for _, name := range strings.Fields(p) {
					props[name] = append(props[name], propValue(c))
				}
				continue
			}
			collect(c)
		}
	}
	collect(scope)

	var d Draft
	if v := props["name"]; len(v) > 0 {
		d.Title = plain(v[0])
	}
	if v := props["recipeYield"]; len(v) > 0 {
		d.Servings = yieldOf(v[0])
	}
	d.Ingredients = ingredientLines(append(props["recipeIngredient"], props["ingredients"]...))
	for _, s := range props["recipeInstructions"] {
		d.Steps = append(d.Steps, lines(s)...)
	}
	for _, s := range append(props["keywords"], props["recipeCategory"]...) {
		for _, p := range strings.Split(s, ",") {
			if p = plain(p); p != "" {
				d.Tags = append(d.Tags, p)
			}
		}
	}
	return d, true
}

// --- visible text ---

func pageTitle(doc *xhtml.Node) string {
	var h1, title string
	walk(doc, func(n *xhtml.Node) bool {
		if n.Type == xhtml.ElementNode {
			switch n.Data {
			case "h1":
				if h1 == "" {
					h1 = plain(blockText(n))
				}
			case "title":
				if title == "" && n.FirstChild != nil {
					title = plain(n.FirstChild.Data)
				}
			}
		}
		return true
	})
	if h1 != "" {
		return h1
	}
	return title
}

// fromPageText is the last resort. It only accepts pages that clearly contain both parts of a recipe,
// otherwise navigation and prose would turn into a bogus recipe.
func fromPageText(doc *xhtml.Node) (Draft, error) {
	d := ParseText(blockText(doc))
	if len(d.Ingredients) < 2 || len(d.Steps) < 1 {
		return Draft{}, ErrUnparsable
	}
	if t := pageTitle(doc); t != "" {
		d.Title = t
	}
	d.Warnings = append([]string{"Dane strukturalne nie zostały znalezione, wynik opiera się na tekście strony. Sprawdź go dokładnie."}, d.Warnings...)
	return d, nil
}
```

`internal/importer/service.go`:

```go
package importer

import (
	"context"
	"strings"
)

type Service struct {
	fetcher *Fetcher
}

func New() *Service { return &Service{fetcher: NewFetcher()} }

// Import turns a link or pasted text into a reviewed-ready Draft. It never saves anything.
func (s *Service) Import(ctx context.Context, src Source) (Draft, error) {
	switch src.Kind {
	case KindText:
		return importText(src.Text)
	case KindURL:
		return s.importURL(ctx, src.URL)
	}
	return Draft{}, invalid("nieznany rodzaj źródła")
}

func importText(text string) (Draft, error) {
	if len(text) > MaxTextBytes {
		return Draft{}, invalid("tekst może mieć najwyżej %d KB", MaxTextBytes>>10)
	}
	if strings.TrimSpace(text) == "" {
		return Draft{}, invalid("tekst przepisu jest pusty")
	}
	d := ParseText(text)
	if len(d.Ingredients)+len(d.Steps) == 0 {
		return Draft{}, ErrUnparsable
	}
	d.finish()
	return d, nil
}

func (s *Service) importURL(ctx context.Context, raw string) (Draft, error) {
	body, err := s.fetcher.Fetch(ctx, raw)
	if err != nil {
		return Draft{}, err
	}
	d, err := ParseHTML(body)
	if err != nil {
		return Draft{}, err
	}
	d.SourceURL = strings.TrimSpace(raw)
	d.finish()
	return d, nil
}
```

- [ ] **Step 5: Tidy and run the tests**

Run: `.superpowers/bin/dkgo go mod tidy && .superpowers/bin/dkgo gofmt -l internal && .superpowers/bin/dkgo go vet ./... && .superpowers/bin/dkgo go test ./internal/importer/`
Expected: PASS. Jeśli `TestParseHTMLJSONLDGraph` zgłasza inne tagi, sprawdź kolejność `append(ldTags(keywords), ldTags(recipeCategory)...)` oraz małe litery w `finish`.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/importer
git commit -m "feat(importer): extract recipes from web pages and expose Service

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Endpointy HTTP, limit częstotliwości, OpenAPI

**Files:**
- Create: `internal/httpapi/importer.go`
- Modify: `internal/httpapi/server.go` (Deps, Server, trasy), `internal/httpapi/errors.go` (mapowanie błędów), `internal/httpapi/server_test.go` (`newEnv`, struktura `env`), `cmd/mealplan/main.go`, `api/openapi.yaml`
- Test: `internal/httpapi/importer_test.go`

**Interfaces:**
- Consumes: `importer.Source`, `importer.Draft`, błędy `importer.Err*`, `auth.RateLimitedError`, `principalFrom(r).UserID`.
- Produces:
  - `type Importer interface{ Import(ctx context.Context, src importer.Source) (importer.Draft, error) }` w `httpapi`; `Deps.Importer Importer`
  - `POST /api/v1/recipes/import/url` `{url}` i `POST /api/v1/recipes/import/text` `{text}` → `200` `draftJSON`
  - błędy: `422 import_blocked`, `422 import_unparsable`, `422 import_failed`, `504 import_timeout`, `429 rate_limited` (tylko URL)
  - JSON szkicu: `{title, servings, sourceUrl, tags[], ingredients[{raw,name,quantity|null,unit,note,confidence}], steps[], warnings[]}`

- [ ] **Step 1: Write the failing tests**

Zmień `internal/httpapi/server_test.go`: dodaj do struktury `env` pole `imp *fakeImporter`, a w `newEnv` zamień linię z `deps` na:

```go
	imp := &fakeImporter{}
	deps := httpapi.Deps{Recipes: recipes, Plan: plan.New(st, st), Shopping: shopping.New(st, st), Importer: imp}
```

i zwróć `&env{srv: srv, st: st, svc: svc, clock: clock, imp: imp}`.

`internal/httpapi/importer_test.go`:

```go
package httpapi_test

import (
	"context"
	"strings"
	"testing"

	"mealplan/internal/domain"
	"mealplan/internal/importer"
)

type fakeImporter struct {
	draft importer.Draft
	err   error
	calls []importer.Source
}

func (f *fakeImporter) Import(_ context.Context, src importer.Source) (importer.Draft, error) {
	f.calls = append(f.calls, src)
	return f.draft, f.err
}

func qty(v float64) *float64 { return &v }

type draftResp struct {
	Title       string   `json:"title"`
	Servings    int      `json:"servings"`
	SourceURL   string   `json:"sourceUrl"`
	Tags        []string `json:"tags"`
	Ingredients []struct {
		Raw        string   `json:"raw"`
		Name       string   `json:"name"`
		Quantity   *float64 `json:"quantity"`
		Unit       string   `json:"unit"`
		Note       string   `json:"note"`
		Confidence string   `json:"confidence"`
	} `json:"ingredients"`
	Steps    []string `json:"steps"`
	Warnings []string `json:"warnings"`
}

func sampleDraft() importer.Draft {
	return importer.Draft{
		Title: "Zupa", Servings: 4, SourceURL: "https://example.com/z", Tags: []string{"zupa"},
		Ingredients: []importer.IngredientLine{
			{Raw: "200 g mąki", Name: "mąki", Quantity: qty(200), Unit: "g", Confidence: "ok"},
			{Raw: "sól do smaku", Name: "sól", Note: "do smaku", Confidence: "ok"},
		},
		Steps:    []string{"Gotuj."},
		Warnings: []string{"uwaga"},
	}
}

func TestImportURLReturnsDraftAndSavesNothing(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	e.imp.draft = sampleDraft()

	rec := c.do("POST", "/api/v1/recipes/import/url", map[string]any{"url": "https://example.com/z"})
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var d draftResp
	decode(t, rec, &d)
	if d.Title != "Zupa" || d.Servings != 4 || len(d.Ingredients) != 2 || d.Ingredients[0].Quantity == nil || *d.Ingredients[0].Quantity != 200 ||
		d.Ingredients[1].Quantity != nil || d.Ingredients[1].Note != "do smaku" || d.Warnings[0] != "uwaga" {
		t.Fatalf("%+v", d)
	}
	if len(e.imp.calls) != 1 || e.imp.calls[0].Kind != importer.KindURL || e.imp.calls[0].URL != "https://example.com/z" {
		t.Fatalf("calls=%+v", e.imp.calls)
	}
	var list []recipeJSON
	decode(t, c.do("GET", "/api/v1/recipes", nil), &list)
	if len(list) != 0 {
		t.Fatalf("import must not save: %+v", list)
	}
}

func TestImportTextReturnsDraft(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	e.imp.draft = sampleDraft()
	rec := c.do("POST", "/api/v1/recipes/import/text", map[string]any{"text": "Zupa\n1 l wody"})
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(e.imp.calls) != 1 || e.imp.calls[0].Kind != importer.KindText || e.imp.calls[0].Text != "Zupa\n1 l wody" {
		t.Fatalf("calls=%+v", e.imp.calls)
	}
}

func TestImportEmptySlicesAreArraysNotNull(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	e.imp.draft = importer.Draft{Title: "x"}
	rec := c.do("POST", "/api/v1/recipes/import/text", map[string]any{"text": "x"})
	body := rec.Body.String()
	for _, want := range []string{`"tags":[]`, `"ingredients":[]`, `"steps":[]`, `"warnings":[]`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in %s", want, body)
		}
	}
}

func TestImportErrorMapping(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{importer.ErrBlocked, 422, "import_blocked"},
		{importer.ErrUnparsable, 422, "import_unparsable"},
		{importer.ErrFetch, 422, "import_failed"},
		{importer.ErrTimeout, 504, "import_timeout"},
		{domain.ErrInvalid, 422, "invalid_input"},
	}
	for _, tc := range cases {
		e.imp.err = tc.err
		rec := c.do("POST", "/api/v1/recipes/import/text", map[string]any{"text": "x"})
		if rec.Code != tc.status || errCode(t, rec) != tc.code {
			t.Errorf("%v: %d %s", tc.err, rec.Code, rec.Body)
		}
	}
}

func TestImportRequiresSessionCSRFAndKnownFields(t *testing.T) {
	e := newEnv(t)
	anon := e.client(t)
	if rec := anon.do("POST", "/api/v1/recipes/import/text", map[string]any{"text": "x"}); rec.Code != 401 {
		t.Errorf("anonymous: %d", rec.Code)
	}
	c := e.client(t)
	setup(t, c)
	if rec := c.do("POST", "/api/v1/recipes/import/text", map[string]any{"text": "x", "extra": 1}); rec.Code != 400 {
		t.Errorf("unknown field: %d", rec.Code)
	}
}

func TestImportURLIsRateLimitedPerUser(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	e.imp.draft = sampleDraft()
	for i := 0; i < 10; i++ {
		if rec := c.do("POST", "/api/v1/recipes/import/url", map[string]any{"url": "https://example.com/z"}); rec.Code != 200 {
			t.Fatalf("call %d: %d", i, rec.Code)
		}
	}
	rec := c.do("POST", "/api/v1/recipes/import/url", map[string]any{"url": "https://example.com/z"})
	if rec.Code != 429 || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("11th call: %d %v", rec.Code, rec.Header())
	}
	if rec := c.do("POST", "/api/v1/recipes/import/text", map[string]any{"text": "x"}); rec.Code != 200 {
		t.Errorf("text import must not be limited: %d", rec.Code)
	}
}
```

Dodatkowo test jednostkowy limitera (wewnętrzny, nowy plik `internal/httpapi/limiter_test.go`, pakiet `httpapi`):

```go
package httpapi

import (
	"testing"
	"time"
)

func TestLimiterWindowSlides(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	l := newLimiter(2, time.Minute)
	l.now = func() time.Time { return now }
	for i := 0; i < 2; i++ {
		if ok, _ := l.allow("u"); !ok {
			t.Fatalf("call %d rejected", i)
		}
	}
	ok, retry := l.allow("u")
	if ok || retry != time.Minute {
		t.Fatalf("ok=%v retry=%v", ok, retry)
	}
	if ok, _ := l.allow("other"); !ok {
		t.Error("another user must have its own budget")
	}
	now = now.Add(61 * time.Second)
	if ok, _ := l.allow("u"); !ok {
		t.Error("budget must return after the window")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `.superpowers/bin/dkgo go test ./internal/httpapi/`
Expected: FAIL (build: `Deps.Importer`, `newLimiter` nie istnieją).

- [ ] **Step 3: Write the implementation**

`internal/httpapi/importer.go`:

```go
package httpapi

import (
	"context"
	"net/http"
	"sync"
	"time"

	"mealplan/internal/auth"
	"mealplan/internal/importer"
)

// Importer turns a link or pasted text into a recipe draft. It never saves anything.
type Importer interface {
	Import(ctx context.Context, src importer.Source) (importer.Draft, error)
}

type draftIngredientJSON struct {
	Raw        string   `json:"raw"`
	Name       string   `json:"name"`
	Quantity   *float64 `json:"quantity"`
	Unit       string   `json:"unit"`
	Note       string   `json:"note"`
	Confidence string   `json:"confidence"`
}

type draftJSON struct {
	Title       string                `json:"title"`
	Servings    int                   `json:"servings"`
	SourceURL   string                `json:"sourceUrl"`
	Tags        []string              `json:"tags"`
	Ingredients []draftIngredientJSON `json:"ingredients"`
	Steps       []string              `json:"steps"`
	Warnings    []string              `json:"warnings"`
}

func toDraftJSON(d importer.Draft) draftJSON {
	out := draftJSON{Title: d.Title, Servings: d.Servings, SourceURL: d.SourceURL,
		Tags: orEmpty(d.Tags), Steps: orEmpty(d.Steps), Warnings: orEmpty(d.Warnings),
		Ingredients: []draftIngredientJSON{}}
	for _, i := range d.Ingredients {
		out.Ingredients = append(out.Ingredients, draftIngredientJSON{Raw: i.Raw, Name: i.Name, Quantity: i.Quantity,
			Unit: i.Unit, Note: i.Note, Confidence: i.Confidence})
	}
	return out
}

type importURLJSON struct {
	URL string `json:"url"`
}

type importTextJSON struct {
	Text string `json:"text"`
}

func (s *Server) handleImportURL(w http.ResponseWriter, r *http.Request) {
	var in importURLJSON
	if !decode(w, r, &in) {
		return
	}
	if ok, retry := s.importLimit.allow(principalFrom(r).UserID); !ok {
		s.fail(w, r, &auth.RateLimitedError{RetryAfter: retry})
		return
	}
	d, err := s.deps.Importer.Import(r.Context(), importer.Source{Kind: importer.KindURL, URL: in.URL})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDraftJSON(d))
}

func (s *Server) handleImportText(w http.ResponseWriter, r *http.Request) {
	var in importTextJSON
	if !decode(w, r, &in) {
		return
	}
	d, err := s.deps.Importer.Import(r.Context(), importer.Source{Kind: importer.KindText, Text: in.Text})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDraftJSON(d))
}

// limiter allows max calls per key in a sliding window. It keeps the server from being used to probe the network.
type limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	max    int
	window time.Duration
	now    func() time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{hits: map[string][]time.Time{}, max: max, window: window, now: time.Now}
}

func (l *limiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(now.Add(-l.window)) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false, kept[0].Add(l.window).Sub(now)
	}
	l.hits[key] = append(kept, now)
	return true, 0
}
```

`internal/httpapi/server.go`:
- w `Deps` dodaj `Importer Importer`;
- w `Server` dodaj `importLimit *limiter`;
- w `New` ustaw `importLimit: newLimiter(10, time.Minute)` (dodaj import `"time"`);
- w `routes()` dodaj po `DELETE /api/v1/recipes/{id}`:

```go
		{"POST /api/v1/recipes/import/url", s.private(s.handleImportURL)},
		{"POST /api/v1/recipes/import/text", s.private(s.handleImportText)},
```

`internal/httpapi/errors.go`: w `fail` dodaj przed `case errors.Is(err, domain.ErrInvalid)` (importer.ErrInvalid owija domain.ErrInvalid, ale błędy importera to osobne wartości, więc kolejność nie ma znaczenia; zachowaj grupowo):

```go
	case errors.Is(err, importer.ErrBlocked):
		writeError(w, http.StatusUnprocessableEntity, "import_blocked", "this address cannot be imported")
	case errors.Is(err, importer.ErrUnparsable):
		writeError(w, http.StatusUnprocessableEntity, "import_unparsable", "no recipe found in the source")
	case errors.Is(err, importer.ErrFetch):
		writeError(w, http.StatusUnprocessableEntity, "import_failed", "the page could not be fetched")
	case errors.Is(err, importer.ErrTimeout):
		writeError(w, http.StatusGatewayTimeout, "import_timeout", "the page took too long to respond")
```

i dodaj import `"mealplan/internal/importer"`.

`cmd/mealplan/main.go`: w `httpapi.Deps{…}` dodaj `Importer: importer.New()` i import `"mealplan/internal/importer"`.

`api/openapi.yaml`: dodaj po bloku `/api/v1/recipes/{id}:` (czyli przed `/api/v1/plan/{week}:`):

```yaml
  /api/v1/recipes/import/url:
    post:
      summary: Fetch a web page and return a recipe draft (nothing is saved)
      description: Limited to 10 requests per minute per user. Only http(s) on default ports; private, loopback, link-local and Tailscale addresses are refused.
      security: [{ session: [], csrf: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/ImportUrlInput" }
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: { $ref: "#/components/schemas/RecipeDraft" }
        "400": { $ref: "#/components/responses/BadRequest" }
        "401": { $ref: "#/components/responses/Unauthorized" }
        "403": { $ref: "#/components/responses/Csrf" }
        "413": { $ref: "#/components/responses/TooLarge" }
        "422": { $ref: "#/components/responses/Unprocessable" }
        "429": { description: Too many imports; see Retry-After (code rate_limited) }
        "504": { description: The page took too long to respond (code import_timeout) }
  /api/v1/recipes/import/text:
    post:
      summary: Parse pasted recipe text and return a recipe draft (nothing is saved)
      security: [{ session: [], csrf: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/ImportTextInput" }
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: { $ref: "#/components/schemas/RecipeDraft" }
        "400": { $ref: "#/components/responses/BadRequest" }
        "401": { $ref: "#/components/responses/Unauthorized" }
        "403": { $ref: "#/components/responses/Csrf" }
        "413": { $ref: "#/components/responses/TooLarge" }
        "422": { $ref: "#/components/responses/Unprocessable" }
```

oraz w `components.schemas` (po `Recipe`):

```yaml
    ImportUrlInput:
      type: object
      required: [url]
      properties:
        url: { type: string, maxLength: 2000 }
    ImportTextInput:
      type: object
      required: [text]
      properties:
        text: { type: string, description: Up to 100 KB }
    DraftIngredient:
      type: object
      required: [raw, name, quantity, unit, note, confidence]
      properties:
        raw: { type: string, description: The line as found in the source }
        name: { type: string, description: Empty when confidence is unparsed }
        quantity: { type: number, nullable: true }
        unit: { type: string }
        note: { type: string, description: Preparation hint such as "posiekana"; shown in the preview, not stored }
        confidence: { type: string, enum: [ok, guess, unparsed] }
    RecipeDraft:
      type: object
      required: [title, servings, sourceUrl, tags, ingredients, steps, warnings]
      properties:
        title: { type: string }
        servings: { type: integer, description: 0 when unknown }
        sourceUrl: { type: string }
        tags: { type: array, items: { type: string } }
        ingredients: { type: array, items: { $ref: "#/components/schemas/DraftIngredient" } }
        steps: { type: array, items: { type: string } }
        warnings: { type: array, items: { type: string } }
```

Błędy kodów (`import_blocked`, `import_unparsable`, `import_failed`) są opisane w `Unprocessable`; dopisz je do jego `description`, czytając istniejący tekst.

- [ ] **Step 4: Run tests to verify they pass**

Run: `.superpowers/bin/dkgo gofmt -l . ; .superpowers/bin/dkgo go vet ./... && .superpowers/bin/dkgo go test ./...`
Expected: PASS, w tym `TestRoutesMatchOpenAPI`. Testy Postgresa mogą być pomijane, jeśli nie ma bazy; odnotuj to w raporcie.

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi cmd api
git commit -m "feat(api): recipe import endpoints with per-user rate limit

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Frontend: import, podgląd w edytorze, duplikaty

**Files:**
- Modify: `web/src/api/types.ts`, `web/src/stores/recipes.ts`, `web/src/messages.ts`, `web/src/router.ts`, `web/src/views/RecipesView.vue`, `web/src/views/RecipeEditView.vue`
- Create: `web/src/importDraft.ts`, `web/src/importDraft.test.ts`, `web/src/stores/import.ts`, `web/src/stores/import.test.ts`, `web/src/views/ImportView.vue`
- Test: `web/src/stores/recipes.test.ts` (dopisek)

**Interfaces:**
- Consumes: API z Task 6 (`POST /recipes/import/url|text`), `api()` z `web/src/api/client.ts`, `useRecipes`.
- Produces:
  - typy `Confidence`, `DraftIngredient`, `RecipeDraft`; `Ingredient` zyskuje opcjonalne `raw?`, `note?`, `confidence?`
  - `draftToRecipe(d: RecipeDraft, id?: string): Recipe`, `normalizeUrl(u: string): string`, `findDuplicate(d: RecipeDraft, existing: Recipe[]): Recipe | undefined` w `importDraft.ts`
  - `useImport()` (Pinia): stan `draft: RecipeDraft | null`, `targetId: string`; akcje `fromUrl(url)`, `fromText(text)`, `clear()`
  - trasa `recipe-import` (`/recipes/import`)

- [ ] **Step 1: Write the failing tests**

`web/src/importDraft.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { draftToRecipe, findDuplicate, normalizeUrl } from './importDraft'
import type { Recipe, RecipeDraft } from './api/types'

function draft(over: Partial<RecipeDraft> = {}): RecipeDraft {
  return {
    title: 'Zupa',
    servings: 4,
    sourceUrl: 'https://Example.com/zupa/',
    tags: ['zupa'],
    ingredients: [
      { raw: '1 l wody', name: 'wody', quantity: 1, unit: 'l', note: '', confidence: 'ok' },
      { raw: '???', name: '', quantity: null, unit: '', note: '', confidence: 'unparsed' },
    ],
    steps: ['Gotuj.'],
    warnings: [],
    ...over,
  }
}

function recipe(over: Partial<Recipe>): Recipe {
  return { id: 'r1', title: 'Inna', baseServings: 4, sourceUrl: '', tags: [], ingredients: [], steps: [], ...over }
}

describe('draftToRecipe', () => {
  it('maps a draft and keeps per-row import hints', () => {
    const r = draftToRecipe(draft())
    expect(r.id).toBe('')
    expect(r.baseServings).toBe(4)
    expect(r.ingredients[0]).toMatchObject({ name: 'wody', quantity: 1, unit: 'l', confidence: 'ok' })
  })
  it('defaults unknown servings to 4 and keeps the id when updating', () => {
    const r = draftToRecipe(draft({ servings: 0 }), 'abc')
    expect(r.baseServings).toBe(4)
    expect(r.id).toBe('abc')
  })
  it('never loses an unparsed line: the raw text becomes the name, cut to 200 characters', () => {
    const r = draftToRecipe(draft({ ingredients: [{ raw: 'x'.repeat(300), name: '', quantity: null, unit: '', note: '', confidence: 'unparsed' }] }))
    expect(r.ingredients[0].name).toHaveLength(200)
  })
  it('copies arrays so editing the recipe does not change the draft', () => {
    const d = draft()
    const r = draftToRecipe(d)
    r.steps.push('x')
    r.tags.push('y')
    expect(d.steps).toHaveLength(1)
    expect(d.tags).toHaveLength(1)
  })
})

describe('normalizeUrl', () => {
  it('ignores scheme, www, case, fragment and trailing slashes', () => {
    expect(normalizeUrl('HTTPS://www.Example.com/Zupa/#kroki')).toBe('example.com/zupa')
    expect(normalizeUrl('http://example.com/zupa')).toBe('example.com/zupa')
    expect(normalizeUrl('')).toBe('')
  })
})

describe('findDuplicate', () => {
  it('matches by source address or by title, ignoring case', () => {
    const byUrl = recipe({ id: 'a', sourceUrl: 'http://www.example.com/zupa' })
    const byTitle = recipe({ id: 'b', title: ' zupa ' })
    expect(findDuplicate(draft(), [recipe({ id: 'x' }), byUrl])?.id).toBe('a')
    expect(findDuplicate(draft({ sourceUrl: '' }), [recipe({ id: 'x' }), byTitle])?.id).toBe('b')
  })
  it('does not match unrelated recipes or empty values', () => {
    expect(findDuplicate(draft({ sourceUrl: '', title: '' }), [recipe({ title: '', sourceUrl: '' })])).toBeUndefined()
    expect(findDuplicate(draft(), [recipe({})])).toBeUndefined()
  })
})
```

`web/src/stores/import.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useImport } from './import'

const sample = { title: 'Zupa', servings: 2, sourceUrl: '', tags: [], ingredients: [], steps: [], warnings: [] }

describe('useImport', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  function mockFetch(status: number, body: unknown) {
    const spy = vi.fn(async (_url: string, _init?: RequestInit) => new Response(JSON.stringify(body), { status }))
    vi.stubGlobal('fetch', spy)
    return spy
  }

  it('posts a link and keeps the draft', async () => {
    const spy = mockFetch(200, sample)
    const s = useImport()
    const d = await s.fromUrl('https://example.com/z')
    expect(d.title).toBe('Zupa')
    expect(s.draft?.servings).toBe(2)
    const [url, init] = spy.mock.calls[0]
    expect(url).toBe('/api/v1/recipes/import/url')
    expect(JSON.parse(String(init?.body))).toEqual({ url: 'https://example.com/z' })
  })

  it('posts pasted text', async () => {
    const spy = mockFetch(200, sample)
    await useImport().fromText('Zupa\n1 l wody')
    expect(spy.mock.calls[0][0]).toBe('/api/v1/recipes/import/text')
  })

  it('leaves no draft behind when the import fails and clear() resets everything', async () => {
    mockFetch(422, { code: 'import_unparsable', message: 'x' })
    const s = useImport()
    await expect(s.fromText('x')).rejects.toMatchObject({ code: 'import_unparsable' })
    expect(s.draft).toBeNull()
    mockFetch(200, sample)
    await s.fromText('x')
    s.targetId = 'r1'
    s.clear()
    expect(s.draft).toBeNull()
    expect(s.targetId).toBe('')
  })
})
```

Dopisz do `web/src/stores/recipes.test.ts` (w bloku `describe('useRecipes'`, obok innych testów `save`; skopiuj styl `mockFetch` stamtąd):

```ts
  it('sends only schema fields when saving an imported recipe', async () => {
    const spy = mockFetch(201, { id: 'n' })
    const s = useRecipes()
    await s.save({
      id: '', title: 'Zupa', baseServings: 4, sourceUrl: '', tags: [], steps: [],
      ingredients: [{ name: 'woda', quantity: 1, unit: 'l', raw: '1 l wody', note: 'zimna', confidence: 'guess' }],
    })
    const body = JSON.parse(String(spy.mock.calls[0][1]?.body))
    expect(body.ingredients[0]).toEqual({ name: 'woda', quantity: 1, unit: 'l' })
  })
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `.superpowers/bin/dknode npm test`
Expected: FAIL (brak `importDraft.ts`, `stores/import.ts`; zapis wysyła `raw/note/confidence`).

- [ ] **Step 3: Write the implementation**

`web/src/api/types.ts`: zastąp `Ingredient` i dodaj typy szkicu:

```ts
export type Confidence = 'ok' | 'guess' | 'unparsed'

export interface Ingredient {
  name: string
  quantity: number | null
  unit: string
  // Set only on rows that came from an import; used for highlighting, never sent to the API.
  raw?: string
  note?: string
  confidence?: Confidence
}

export interface DraftIngredient extends Ingredient {
  raw: string
  note: string
  confidence: Confidence
}

export interface RecipeDraft {
  title: string
  servings: number
  sourceUrl: string
  tags: string[]
  ingredients: DraftIngredient[]
  steps: string[]
  warnings: string[]
}
```

`web/src/importDraft.ts`:

```ts
import type { Recipe, RecipeDraft } from './api/types'

export function draftToRecipe(d: RecipeDraft, id = ''): Recipe {
  return {
    id,
    title: d.title,
    baseServings: d.servings > 0 ? d.servings : 4,
    sourceUrl: d.sourceUrl,
    tags: [...d.tags],
    // An unparsed line keeps its text as the name so nothing is lost; the highlighted row asks the user to fix it.
    ingredients: d.ingredients.map((i) => ({
      name: i.confidence === 'unparsed' && !i.name ? Array.from(i.raw).slice(0, 200).join('') : i.name,
      quantity: i.quantity,
      unit: i.unit,
      raw: i.raw,
      note: i.note,
      confidence: i.confidence,
    })),
    steps: [...d.steps],
  }
}

export function normalizeUrl(u: string): string {
  return u
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, '')
    .replace(/^www\./, '')
    .replace(/#.*$/, '')
    .replace(/\/+$/, '')
}

export function findDuplicate(d: RecipeDraft, existing: Recipe[]): Recipe | undefined {
  const url = normalizeUrl(d.sourceUrl)
  const title = d.title.trim().toLowerCase()
  return existing.find(
    (r) => (url !== '' && normalizeUrl(r.sourceUrl) === url) || (title !== '' && r.title.trim().toLowerCase() === title),
  )
}
```

`web/src/stores/import.ts`:

```ts
import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { RecipeDraft } from '../api/types'

// Holds the draft between the import screen and the recipe editor.
export const useImport = defineStore('import', {
  state: () => ({ draft: null as RecipeDraft | null, targetId: '' }),
  actions: {
    async fromUrl(url: string): Promise<RecipeDraft> {
      this.draft = null
      this.draft = await api<RecipeDraft>('POST', '/recipes/import/url', { url })
      return this.draft
    },
    async fromText(text: string): Promise<RecipeDraft> {
      this.draft = null
      this.draft = await api<RecipeDraft>('POST', '/recipes/import/text', { text })
      return this.draft
    },
    clear() {
      this.draft = null
      this.targetId = ''
    },
  },
})
```

`web/src/stores/recipes.ts`: w `save` zamień mapowanie składników na jawne pola:

```ts
        // v-model.number yields '' when the field is cleared; the API expects null.
        // Only schema fields are sent: imported rows carry extra hints the server rejects.
        ingredients: r.ingredients.map((i) => ({
          name: i.name,
          quantity: (i.quantity as unknown) === '' ? null : i.quantity,
          unit: i.unit,
        })),
```

`web/src/messages.ts`: w obiekcie `t` dopisz (przed zamykającym `}` obiektu `t`, po `account`):

```ts
  importRecipe: 'Importuj',
  importTitle: 'Importuj przepis',
  importLink: 'Link',
  importText: 'Tekst',
  importUrlLabel: 'Adres przepisu',
  importUrlHint: 'Wklej link do strony z przepisem.',
  importTextLabel: 'Wklej przepis',
  importTextHint: 'Tytuł, składniki i kroki w dowolnej formie.',
  analyze: 'Analizuj',
  importDuplicateTitle: 'Taki przepis już istnieje',
  importDuplicateHint: 'Znaleziono przepis o tym samym adresie lub tytule:',
  importUpdate: 'Zaktualizuj istniejący',
  importAddNew: 'Dodaj jako nowy',
  importedBanner: 'Przepis wczytany z importu. Sprawdź wiersze oznaczone kolorem, zanim go zapiszesz.',
  fromText: 'Z tekstu',
  ingredientNote: 'Notatka (nie jest zapisywana)',
```

oraz w mapie `errors` (przed `cookie_not_stored`):

```ts
  import_blocked: 'Ten adres jest niedozwolony (sieć lokalna lub prywatna).',
  import_unparsable: 'Nie udało się odczytać przepisu. Spróbuj wkleić jego tekst w zakładce „Tekst".',
  import_failed: 'Nie udało się pobrać strony. Sprawdź adres albo wklej tekst przepisu.',
  import_timeout: 'Strona odpowiada zbyt wolno. Spróbuj ponownie lub wklej tekst przepisu.',
```

`web/src/router.ts`: dodaj import `import ImportView from './views/ImportView.vue'` i trasę przed `recipes/new`:

```ts
        { path: 'recipes/import', name: 'recipe-import', component: ImportView },
```

`web/src/views/RecipesView.vue`: zastąp przycisk w nagłówku dwoma:

```vue
    <v-spacer />
    <v-btn variant="tonal" :to="{ name: 'recipe-import' }" prepend-icon="mdi-download" class="mr-2">{{ t.importRecipe }}</v-btn>
    <v-btn color="primary" :to="{ name: 'recipe-new' }" prepend-icon="mdi-plus">{{ t.newRecipe }}</v-btn>
```

`web/src/views/ImportView.vue`:

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { findDuplicate } from '../importDraft'
import { useImport } from '../stores/import'
import { useRecipes } from '../stores/recipes'
import type { Recipe } from '../api/types'
import { errorText, t } from '../messages'

const router = useRouter()
const imports = useImport()
const recipes = useRecipes()
const tab = ref<'url' | 'text'>('url')
const url = ref('')
const text = ref('')
const busy = ref(false)
const error = ref('')
const duplicate = ref<Recipe | null>(null)

onMounted(() => {
  imports.clear()
})

async function analyze() {
  error.value = ''
  duplicate.value = null
  busy.value = true
  try {
    const draft = tab.value === 'url' ? await imports.fromUrl(url.value) : await imports.fromText(text.value)
    if (!recipes.loaded) await recipes.load()
    const dup = findDuplicate(draft, recipes.items)
    if (dup) {
      duplicate.value = dup
      return
    }
    await router.push({ name: 'recipe-new' })
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}

async function choose(update: boolean) {
  const dup = duplicate.value
  if (update && dup) {
    imports.targetId = dup.id
    await router.push({ name: 'recipe-edit', params: { id: dup.id } })
  } else {
    imports.targetId = ''
    await router.push({ name: 'recipe-new' })
  }
}
</script>

<template>
  <h1 class="text-h5 mb-4">{{ t.importTitle }}</h1>
  <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>

  <v-card v-if="duplicate" border flat class="pa-4 mb-4">
    <div class="text-subtitle-1 mb-1">{{ t.importDuplicateTitle }}</div>
    <div class="text-body-2 mb-3">{{ t.importDuplicateHint }} <strong>{{ duplicate.title }}</strong></div>
    <div class="d-flex ga-3 flex-wrap">
      <v-btn color="primary" @click="choose(true)">{{ t.importUpdate }}</v-btn>
      <v-btn variant="tonal" @click="choose(false)">{{ t.importAddNew }}</v-btn>
    </div>
  </v-card>

  <v-form v-else @submit.prevent="analyze">
    <v-tabs v-model="tab" class="mb-4">
      <v-tab value="url">{{ t.importLink }}</v-tab>
      <v-tab value="text">{{ t.importText }}</v-tab>
    </v-tabs>
    <v-text-field v-if="tab === 'url'" v-model="url" :label="t.importUrlLabel" :hint="t.importUrlHint" persistent-hint type="url" autofocus />
    <v-textarea v-else v-model="text" :label="t.importTextLabel" :hint="t.importTextHint" persistent-hint rows="12" auto-grow autofocus />
    <div class="d-flex ga-3 mt-4">
      <v-btn type="submit" color="primary" :loading="busy" :disabled="tab === 'url' ? !url.trim() : !text.trim()">{{ t.analyze }}</v-btn>
      <v-btn variant="text" :to="{ name: 'recipes' }">{{ t.cancel }}</v-btn>
    </div>
  </v-form>
</template>
```

`web/src/views/RecipeEditView.vue`:

1. Importy w `<script setup>`: dodaj `onBeforeUnmount` do importu z `vue`, oraz

```ts
import { draftToRecipe } from '../importDraft'
import { useImport } from '../stores/import'
```

2. Po `const recipes = useRecipes()` dodaj:

```ts
const imports = useImport()
```

3. Zastąp `onMounted` całym blokiem:

```ts
onMounted(async () => {
  const d = imports.draft
  if (isNew.value) {
    if (d && !imports.targetId) recipe.value = draftToRecipe(d)
    loaded.value = true
    return
  }
  try {
    const existing = await recipes.get(String(route.params.id))
    recipe.value = d && imports.targetId === existing.id ? draftToRecipe(d, existing.id) : existing
    loaded.value = true
  } catch (e) {
    error.value = errorText(e)
  }
})

// A draft must never leak into the next "new recipe" form.
onBeforeUnmount(() => imports.clear())

const fromImport = computed(() => Boolean(imports.draft) && (isNew.value ? !imports.targetId : imports.targetId === recipe.value.id))
```

4. W szablonie, po `<v-alert v-if="error" …>` dodaj:

```vue
  <v-alert v-if="loaded && fromImport" type="info" variant="tonal" class="mb-4">
    {{ t.importedBanner }}
    <ul v-if="imports.draft?.warnings.length" class="mt-2 ps-4">
      <li v-for="w in imports.draft.warnings" :key="w">{{ w }}</li>
    </ul>
  </v-alert>
```

5. Zastąp pętlę składników:

```vue
    <div v-for="(ing, i) in recipe.ingredients" :key="i" :class="['ing-row', ing.confidence && `ing-${ing.confidence}`]">
      <div class="d-flex ga-2 align-start">
        <v-text-field v-model="ing.name" :label="t.ingredientName" class="flex-grow-1" density="compact" />
        <v-text-field v-model.number="ing.quantity" :label="t.quantity" type="number" min="0.001" step="any" density="compact" style="max-width: 110px" />
        <v-text-field v-model="ing.unit" :label="t.unit" density="compact" style="max-width: 110px" />
        <v-btn icon="mdi-close" variant="text" :title="t.remove" @click="recipe.ingredients.splice(i, 1)" />
      </div>
      <div v-if="ing.confidence && ing.confidence !== 'ok'" class="text-caption text-medium-emphasis pb-2">{{ t.fromText }}: „{{ ing.raw }}"</div>
      <div v-if="ing.note" class="text-caption text-medium-emphasis pb-2">{{ t.ingredientNote }}: {{ ing.note }}</div>
    </div>
```

6. Przycisk "Anuluj" zostaje (czyszczenie robi `onBeforeUnmount`). Na końcu pliku dodaj:

```vue
<style scoped>
.ing-row {
  border-radius: 12px;
  padding: 0 8px;
}
.ing-guess {
  background: rgba(var(--v-theme-warning), 0.1);
}
.ing-unparsed {
  background: rgba(var(--v-theme-error), 0.1);
}
</style>
```

- [ ] **Step 4: Run tests and the build**

Run: `.superpowers/bin/dknode npm test && .superpowers/bin/dknode npm run build && touch web/dist/.gitkeep`
Expected: Vitest PASS (83 poprzednich + nowe), `vue-tsc` czysty, build OK. Jeśli test `messages.test.ts` sprawdza kompletność kodów błędów, dopisz oczekiwane klucze.

- [ ] **Step 5: Verify in the browser**

Zbuduj obraz i uruchom aplikację lokalnie (jak w poprzednich zadaniach: `.superpowers/bin/dkgo go build -o /tmp/mealplan ./cmd/mealplan` i uruchomienie z tymczasową bazą, albo przebudowa kontenera `mealplan`). Sprawdź w przeglądarce (desktop i szerokość telefonu):
1. Przepisy → Importuj → zakładka Tekst: wklej `Placki\nPorcje: 4\nSkładniki\n1 kg ziemniaków\n2 jajka\nsól do smaku\nPrzygotowanie\n1. Zetrzyj.\n2. Smaż.` → Analizuj → edytor z wypełnionymi polami, baner, wiersz `2 jajka` podświetlony (guess), notatka "do smaku" widoczna.
2. Zapisz → przepis na liście. Powtórz import tego samego tekstu → karta duplikatu → "Zaktualizuj istniejący" otwiera edytor tego przepisu z danymi z szkicu.
3. Zakładka Link: `http://127.0.0.1/` → komunikat o niedozwolonym adresie; prawdziwy link z polskiego serwisu z przepisem → szkic.
4. Wejście w "Nowy przepis" po porzuceniu importu → puste pole (szkic nie przecieka).
Zrób zrzuty ekranu i dołącz do raportu; napisz jawnie, czego nie sprawdzono (np. brak dostępu do sieci zewnętrznej).

- [ ] **Step 6: Commit**

```bash
git add web
git commit -m "feat(web): import recipes from a link or pasted text

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Dokumentacja i uzgodnienie specu

**Files:**
- Create: `docs/import.md`
- Modify: `docs/api.md`, `docs/security.md`, `CHANGELOG.md`, `docs/superpowers/specs/2026-11-03-recipe-parser-design.md`

**Interfaces:**
- Consumes: gotowe zachowanie z Tasków 1–7.
- Produces: dokumentacja zgodna z kodem. `docs/docs_test.go` sprawdza bloki mermaid we wszystkich `docs/*.md`: jeśli dodajesz diagram, musi być zamknięty i zaczynać się znanym typem.

- [ ] **Step 1: Write `docs/import.md`**

Napisz po polsku (styl jak sąsiednie dokumenty). Sekcje: (1) co robi import i że niczego nie zapisuje bez zatwierdzenia; (2) Link: kolejność źródeł (JSON-LD, microdata, tekst strony z ostrzeżeniem i warunkiem min. 2 składniki i 1 krok), limity (2 MB, 10 s, 5 przekierowań, porty 80/443, 10 importów/min na użytkownika), kodowania (UTF-8 i starsze strony przez nagłówek/`meta charset`); (3) Tekst: rozpoznawane nagłówki (PL/EN), formy porcji, numeracja i wypunktowania, zasada "każda linia to krok" bez numeracji oraz łączenie zawiniętych linii przy numeracji, limit 100 KB; (4) Linia składnika: tabela przykładów wejście → ilość/jednostka/nazwa/notatka, znaczenie `ok`/`guess`/`unparsed`, `dag` przeliczane na gramy, zakres daje górną wartość, brak ilości daje pustą ilość, notatka nie jest zapisywana; (5) znane ograniczenia (brak lematyzacji, brak czasów przygotowania, strony wymagające JavaScriptu nie zadziałają, wklej wtedy tekst); (6) diagram mermaid przepływu `źródło → Draft → podgląd → POST /recipes`; (7) informacja, że import plików `.md` jest planowany na ten sam pipeline.

- [ ] **Step 2: Update the remaining docs**

- `docs/api.md`: w tabeli zasobów dodaj dwa wiersze `POST /recipes/import/url` i `POST /recipes/import/text` (zwracają szkic, niczego nie zapisują; kody `import_blocked`, `import_unparsable`, `import_failed`, `import_timeout`; limit 10/min dla URL), zachowując styl istniejących wierszy.
- `docs/security.md`: dodaj sekcję "Import z adresu URL (SSRF)" z listą blokowanych zakresów (w tym `100.64.0.0/10`, bo tailnet), kontrolą w dialerze po rozwiązaniu DNS (przekierowania i rebinding), brakiem proxy z otoczenia, ograniczeniem portów, limitami rozmiaru/czasu, limitem częstotliwości, tym że treść błędów sieci nie trafia do odpowiedzi oraz tym, że wynik jest escapowany (bez `v-html`).
- `CHANGELOG.md`: w `## Unreleased` dodaj sekcję `### Import przepisów` z krótkim opisem (link, wklejony tekst, podgląd w edytorze z oznaczaniem niepewnych wierszy, wykrywanie duplikatów, ochrona SSRF, limit 10/min, nowa zależność `golang.org/x/net`).
- Spec `docs/superpowers/specs/2026-11-03-recipe-parser-design.md`: (a) w §6 dodaj kod błędu `422 import_failed` (nie udało się pobrać strony: status inny niż 2xx, zły typ treści, za duży plik, zbyt wiele przekierowań) i limit portów 80/443; (b) w §2/§7 dopisz, że `note` jest tylko podglądem i nie jest zapisywana; (c) w §11 usuń pytanie o duplikaty (decyzja: wchodzą do pierwszej wersji) i dopisz zależność `golang.org/x/net` (parser HTML i dekodowanie stron).

- [ ] **Step 3: Run the whole suite**

Run: `.superpowers/bin/dkgo go test ./... && .superpowers/bin/dknode npm test && .superpowers/bin/dknode npm run build && touch web/dist/.gitkeep`
Expected: wszystko PASS (`docs_test.go` obejmuje diagram z `docs/import.md`).

- [ ] **Step 4: Commit**

```bash
git add docs CHANGELOG.md
git commit -m "docs: recipe import (format, security, API) and spec alignment

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage** (spec → zadanie): §3 pakiet i typy → T2/T5; §4 parser linii → T1, T2; §5 link (SSRF, JSON-LD, microdata, tekst strony) → T4, T5; §5 wklejony tekst → T3; §6 API, kody błędów, limit → T6; §7 UI, podgląd, duplikaty, przycisk → T7; §8 bezpieczeństwo (tabela SSRF, rebinding, escapowanie, limity) → T4, T7 (interpolacja), T8; §9 testy → T2–T7; §10 kolejność i dokumenty → T8. Rozszerzenia względem specu (kod `import_failed`, limit portów, `note` nie zapisywana, `x/net`) są uzgadniane w T8.

**Placeholder scan:** brak "TBD"/"TODO". T8 opisuje treść dokumentów słownie (dokumentacja, nie kod); to zamierzone, bo treść wynika z działającego kodu.

**Type consistency:** `ParseIngredient`, `ParseText`, `ParseHTML`, `Fetcher.Fetch`, `Service.Import`, `Source{Kind,URL,Text}`, stałe `Kind*`/`Conf*` użyte tak samo w T2–T6; JSON `draftJSON` (T6) odpowiada `RecipeDraft` (T7) i schematowi OpenAPI; `newTestFetcher` i `names()`/`fixture()` są zdefiniowane w testach T4/T3/T5 i używane w tym samym pakiecie.

**Ryzyka znane z góry:** (1) tabela w T2 jest specyfikacją zachowania, ale pisana bez uruchomienia, więc implementer może znaleźć 1–3 wiersze do uzgodnienia z kodem; zasada: popraw kod, a wiersz zmieniaj tylko wbrew §4 i opisz w commicie. (2) Test `localhost` w T4 wymaga rozwiązywania nazw w kontenerze. (3) Testy Postgresa i weryfikacja w przeglądarce z prawdziwym serwisem zależą od środowiska i muszą być jawnie raportowane, jeśli pominięte.
