# Mealplan Etap 1: Fundament Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dostarczyć działający szkielet Mealplan: jeden plik binarny Go z osadzonym frontendem Vue, bazą (SQLite, gotowe pod Postgres), logowaniem hasłem, sesjami, zaproszeniami i podstawową powłoką UI w Material Design.

**Architecture:** Backend Go (`net/http`, ServeMux z wzorcami Go 1.22) z pakietami `config`, `domain`, `store`, `auth`, `httpapi`, `spa`. Wszystkie dane domenowe są powiązane z `household_id`. Frontend Vue 3 + Vuetify 3 budowany do `web/dist` i osadzany przez `go:embed`. Kontrakt API opisany w `api/openapi.yaml` i pilnowany testem.

**Tech Stack:** Go 1.23, `modernc.org/sqlite`, `github.com/jackc/pgx/v5` (stdlib), `golang.org/x/crypto/argon2`, Vue 3.5, Vite 5, Vuetify 3, Pinia, Vue Router 4, Vitest, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-11-01-mealplan-design.md`

## Global Constraints

- Moduł Go: `module mealplan`, `go 1.23`. Przy publikacji ścieżkę modułu można zmienić jednym `go mod edit`.
- API: REST/JSON pod `/api/v1`, błędy zawsze w formacie `{"code","message","details"}`.
- Hasła: argon2id (domyślnie m=65536, t=3, p=2), długość 12 do 128 znaków (liczone w runach).
- Tokeny sesji i zaproszeń: 256 bit losowych, w bazie wyłącznie hash SHA-256.
- Ciastko sesji: `HttpOnly`, `Secure` (konfigurowalne), `SameSite=Lax`. CSRF: double-submit (ciastko `mealplan_csrf` + nagłówek `X-CSRF-Token`, oba muszą zgadzać się z tokenem sesji).
- Zaproszenia: jednorazowe, ważne 72 godziny, rola `admin` lub `member`.
- Limity logowania: na login (próg 5 porażek) i na IP (próg 30), narastające blokady do 15 minut.
- Każda tabela domenowa ma `household_id`; zapytania o dane gospodarstwa zawsze go filtrują.
- Konfiguracja wyłącznie zmiennymi środowiskowymi `MEALPLAN_*`. Czasy w bazie jako unix seconds (`BIGINT`). SQL przenośny między SQLite i Postgres (znaki `?` zamieniane na `$n` w `Rebind`).
- Tabela użytkowników nazywa się `app_user` (`user` jest słowem zastrzeżonym w Postgres), reszta nazw jak w specu.
- Licencja MIT. Wszystkie diagramy w dokumentacji w mermaid.
- Teksty UI po polsku, trzymane w jednym pliku `web/src/messages.ts` (pod przyszłe i18n). Komunikaty API po angielsku.
- Poza zakresem Etapu 1 (zostają na Etap 2+): przepisy, plan, zakupy, inwentarz, statystyki, zmiana roli i usuwanie członków, testy Playwright, PWA.

## Review Focus

Wejścia i warunki, które spec implikuje, a nie są oczywiste w testach zadań. Każdy ma test w zadaniu-właścicielu.

1. **Login z wielkimi literami i spacjami** (`"  MARC "` po rejestracji jako `marc`): ma się zalogować (Task 5).
2. **Ogromne hasło lub ciało żądania** (200 znaków, 2 MB JSON): odrzucone bez liczenia argon2, kody `422 password_length` i `413 too_large` (Task 3 i 6).
3. **Dwa równoczesne przyjęcia tego samego zaproszenia**: dokładnie jedno się udaje (Task 4 i 5).
4. **Aplikacja za proxy (Tailscale serve), wszystkie żądania z 127.0.0.1**: limit IP jest hojny, a blokada loginu ma górny limit 15 minut, więc atakujący nie zablokuje konta na stałe (Task 3 i 6).
5. **`COOKIE_SECURE=true` na zwykłym HTTP**: przeglądarka odrzuca ciastko, a użytkownik widzi czytelny komunikat zamiast cichej pętli logowania (Task 7).

---

## File Structure

```
go.mod, go.sum, Makefile, Dockerfile, .gitignore, LICENSE, README.md
cmd/mealplan/main.go                    wiring, graceful shutdown
internal/config/config.go               konfiguracja z env
internal/domain/domain.go               typy domenowe
internal/store/db.go                    Open, Rebind
internal/store/migrate.go               migrator
internal/store/migrations/0001_init.sql schemat
internal/store/store.go                 Store, błędy, helpery
internal/store/users.go                 użytkownicy, gospodarstwa, członkostwa
internal/store/invites.go               zaproszenia
internal/store/sessions.go              sesje i audyt
internal/store/storetest/storetest.go   baza testowa (SQLite lub Postgres)
internal/auth/password.go               argon2id, walidacja hasła
internal/auth/token.go                  tokeny i ID
internal/auth/limiter.go                limiter prób
internal/auth/service.go                logika auth i zaproszeń
internal/httpapi/server.go              router i trasy
internal/httpapi/middleware.go          nagłówki, auth, CSRF, JSON
internal/httpapi/handlers.go            handlery
internal/httpapi/errors.go              format błędów
internal/spa/spa.go                     serwowanie osadzonego frontendu
web/embed.go                            go:embed dist
api/openapi.yaml                        kontrakt
web/                                    aplikacja Vue
docs/, .github/workflows/ci.yml        dokumentacja i CI
```

---

### Task 1: Szkielet repozytorium i konfiguracja

**Files:**
- Create: `go.mod`, `.gitignore`, `LICENSE`, `README.md`, `Makefile`
- Create: `internal/config/config.go`, `internal/config/config_test.go`
- Create: `cmd/mealplan/main.go`

**Interfaces:**
- Produces: `config.Config{Mode Mode; Addr, DBDriver, DBDSN string; CookieSecure bool; SessionIdle, SessionMax time.Duration}`, `config.Load(getenv func(string) string) (Config, error)`, stałe `config.ModeSelfhost`, `config.ModeHosted`.

- [ ] **Step 1: Zainicjuj moduł i pliki repozytorium**

Upewnij się, że repozytorium ma tożsamość git (`git config user.name` i `git config user.email`).

```bash
cd /home/silly/mealplan
go mod init mealplan
go mod edit -go=1.23
```

`.gitignore`:

```
bin/
*.db
*.db-wal
*.db-shm
web/node_modules/
web/dist/*
!web/dist/.gitkeep
```

`LICENSE`:

```
MIT License

Copyright (c) 2026 Mealplan contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

`README.md` (tymczasowy, rozbudowany w Task 9):

```markdown
# Mealplan

Lekka domowa aplikacja do planowania jadłospisu. Licencja MIT.
```

`Makefile`:

```make
.PHONY: web build test run

web:
	cd web && npm ci && npm run build

build: web
	CGO_ENABLED=0 go build -o bin/mealplan ./cmd/mealplan

test:
	go test ./...
	cd web && npm test

run: build
	MEALPLAN_COOKIE_SECURE=false ./bin/mealplan
```

- [ ] **Step 2: Napisz test konfiguracji (ma nie przejść)**

`internal/config/config_test.go`:

```go
package config

import (
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode != ModeSelfhost || c.Addr != ":8080" || c.DBDriver != "sqlite" || !c.CookieSecure {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c.SessionIdle != 7*24*time.Hour || c.SessionMax != 30*24*time.Hour {
		t.Fatalf("unexpected session defaults: %+v", c)
	}
}

func TestLoadOverrides(t *testing.T) {
	c, err := Load(env(map[string]string{
		"MEALPLAN_MODE":          "hosted",
		"MEALPLAN_ADDR":          "127.0.0.1:9000",
		"MEALPLAN_DB_DRIVER":     "postgres",
		"MEALPLAN_DB_DSN":        "postgres://x",
		"MEALPLAN_COOKIE_SECURE": "false",
		"MEALPLAN_SESSION_IDLE":  "1h",
		"MEALPLAN_SESSION_MAX":   "24h",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode != ModeHosted || c.Addr != "127.0.0.1:9000" || c.DBDriver != "postgres" ||
		c.DBDSN != "postgres://x" || c.CookieSecure || c.SessionIdle != time.Hour || c.SessionMax != 24*time.Hour {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func TestLoadRejectsInvalid(t *testing.T) {
	cases := map[string]map[string]string{
		"mode":          {"MEALPLAN_MODE": "cloud"},
		"driver":        {"MEALPLAN_DB_DRIVER": "mysql"},
		"bool":          {"MEALPLAN_COOKIE_SECURE": "maybe"},
		"duration":      {"MEALPLAN_SESSION_IDLE": "soon"},
		"max below idle": {"MEALPLAN_SESSION_IDLE": "48h", "MEALPLAN_SESSION_MAX": "24h"},
	}
	for name, m := range cases {
		if _, err := Load(env(m)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
```

- [ ] **Step 3: Uruchom test i potwierdź, że nie przechodzi**

Run: `go test ./internal/config/ -v`
Expected: FAIL (`undefined: Load`).

- [ ] **Step 4: Zaimplementuj konfigurację**

`internal/config/config.go`:

```go
// Package config loads runtime configuration from MEALPLAN_* environment variables.
package config

import (
	"fmt"
	"strconv"
	"time"
)

type Mode string

const (
	ModeSelfhost Mode = "selfhost"
	ModeHosted   Mode = "hosted"
)

type Config struct {
	Mode         Mode
	Addr         string
	DBDriver     string // "sqlite" or "postgres"
	DBDSN        string
	CookieSecure bool
	SessionIdle  time.Duration
	SessionMax   time.Duration
}

const defaultSQLiteDSN = "file:mealplan.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"

func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Mode:         ModeSelfhost,
		Addr:         ":8080",
		DBDriver:     "sqlite",
		DBDSN:        defaultSQLiteDSN,
		CookieSecure: true,
		SessionIdle:  7 * 24 * time.Hour,
		SessionMax:   30 * 24 * time.Hour,
	}
	if v := getenv("MEALPLAN_MODE"); v != "" {
		c.Mode = Mode(v)
	}
	if c.Mode != ModeSelfhost && c.Mode != ModeHosted {
		return Config{}, fmt.Errorf("MEALPLAN_MODE: unknown mode %q", c.Mode)
	}
	if v := getenv("MEALPLAN_ADDR"); v != "" {
		c.Addr = v
	}
	if v := getenv("MEALPLAN_DB_DRIVER"); v != "" {
		c.DBDriver = v
	}
	if c.DBDriver != "sqlite" && c.DBDriver != "postgres" {
		return Config{}, fmt.Errorf("MEALPLAN_DB_DRIVER: unknown driver %q", c.DBDriver)
	}
	if v := getenv("MEALPLAN_DB_DSN"); v != "" {
		c.DBDSN = v
	}
	if v := getenv("MEALPLAN_COOKIE_SECURE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("MEALPLAN_COOKIE_SECURE: %w", err)
		}
		c.CookieSecure = b
	}
	if err := parseDuration(getenv, "MEALPLAN_SESSION_IDLE", &c.SessionIdle); err != nil {
		return Config{}, err
	}
	if err := parseDuration(getenv, "MEALPLAN_SESSION_MAX", &c.SessionMax); err != nil {
		return Config{}, err
	}
	if c.SessionMax < c.SessionIdle {
		return Config{}, fmt.Errorf("MEALPLAN_SESSION_MAX must not be shorter than MEALPLAN_SESSION_IDLE")
	}
	return c, nil
}

func parseDuration(getenv func(string) string, key string, dst *time.Duration) error {
	v := getenv(key)
	if v == "" {
		return nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fmt.Errorf("%s: invalid duration %q", key, v)
	}
	*dst = d
	return nil
}
```

- [ ] **Step 5: Tymczasowy punkt wejścia**

`cmd/mealplan/main.go` (zostanie zastąpiony w Task 8):

```go
package main

import (
	"log"
	"net/http"
	"os"

	"mealplan/internal/config"
)

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	log.Printf("mealplan (%s) listening on %s", cfg.Mode, cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, mux))
}
```

- [ ] **Step 6: Uruchom testy i build**

Run: `gofmt -w . && go vet ./... && go test ./... && go build ./...`
Expected: testy `ok`.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat: repository skeleton and env-based configuration"
```

---

### Task 2: Warstwa bazy, migracje i schemat

**Files:**
- Create: `internal/store/db.go`, `internal/store/migrate.go`, `internal/store/migrations/0001_init.sql`
- Create: `internal/store/storetest/storetest.go`
- Test: `internal/store/db_test.go`

**Interfaces:**
- Produces:
  - `store.DB{*sql.DB; Driver string}`, `store.Open(driver, dsn string) (*DB, error)` (`driver` to `"sqlite"` lub `"postgres"`), `(*DB).Rebind(q string) string`, `(*DB).Migrate(ctx context.Context) error`.
  - `storetest.Open(t *testing.T) *store.DB` zwraca zmigrowaną, izolowaną bazę. Domyślnie SQLite w katalogu tymczasowym. Gdy `MEALPLAN_TEST_DRIVER=postgres`, używa `MEALPLAN_TEST_DSN` i osobnego schematu na test.

- [ ] **Step 1: Dodaj zależności**

```bash
go get modernc.org/sqlite github.com/jackc/pgx/v5
```

- [ ] **Step 2: Napisz testy (mają nie przejść)**

`internal/store/db_test.go`:

```go
package store_test

import (
	"context"
	"testing"

	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

func TestRebind(t *testing.T) {
	pg := &store.DB{Driver: "postgres"}
	if got := pg.Rebind("SELECT ? , ?"); got != "SELECT $1 , $2" {
		t.Fatalf("postgres rebind: %q", got)
	}
	lite := &store.DB{Driver: "sqlite"}
	if got := lite.Rebind("SELECT ?"); got != "SELECT ?" {
		t.Fatalf("sqlite rebind: %q", got)
	}
}

func TestOpenRejectsUnknownDriver(t *testing.T) {
	if _, err := store.Open("mysql", "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMigrateIsIdempotentAndCreatesTables(t *testing.T) {
	db := storetest.Open(t) // already migrated once
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	for _, table := range []string{"household", "app_user", "membership", "invite", "session", "audit_event"} {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
			t.Errorf("table %s: %v", table, err)
		}
	}
	var applied int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&applied); err != nil || applied != 1 {
		t.Fatalf("schema_migrations: n=%d err=%v", applied, err)
	}
}
```

- [ ] **Step 3: Uruchom test i potwierdź, że nie przechodzi**

Run: `go test ./internal/store/ -v`
Expected: FAIL (brak pakietu / `undefined: store.Open`).

- [ ] **Step 4: Zaimplementuj `db.go`**

```go
// Package store is the persistence layer. SQL is portable between SQLite and Postgres.
package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	Driver string // "sqlite" or "postgres"
}

func Open(driver, dsn string) (*DB, error) {
	var name string
	switch driver {
	case "sqlite":
		name = "sqlite"
	case "postgres":
		name = "pgx"
	default:
		return nil, fmt.Errorf("store: unknown driver %q", driver)
	}
	sdb, err := sql.Open(name, dsn)
	if err != nil {
		return nil, err
	}
	if driver == "sqlite" {
		sdb.SetMaxOpenConns(1) // SQLite allows a single writer; avoids SQLITE_BUSY
	}
	if err := sdb.Ping(); err != nil {
		_ = sdb.Close()
		return nil, err
	}
	return &DB{DB: sdb, Driver: driver}, nil
}

// Rebind converts "?" placeholders to "$n" for Postgres.
func (d *DB) Rebind(q string) string {
	if d.Driver != "postgres" {
		return q
	}
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			b.WriteString("$" + strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
```

- [ ] **Step 5: Zaimplementuj migrator**

`internal/store/migrate.go`:

```go
package store

import (
	"context"
	"embed"
	"fmt"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies pending migrations in filename order, each in its own transaction.
// Migration files are split on ";", so they must not contain ";" inside strings or comments.
func (d *DB) Migrate(ctx context.Context) error {
	if _, err := d.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at BIGINT NOT NULL)`); err != nil {
		return err
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, e := range entries {
		version := strings.TrimSuffix(e.Name(), ".sql")
		var n int
		if err := d.QueryRowContext(ctx, d.Rebind(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`), version).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, stmt := range strings.Split(string(body), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration %s: %w", version, err)
			}
		}
		if _, err := tx.ExecContext(ctx, d.Rebind(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`), version, time.Now().Unix()); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 6: Schemat**

`internal/store/migrations/0001_init.sql`:

```sql
CREATE TABLE household (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at BIGINT NOT NULL
);

CREATE TABLE app_user (
    id TEXT PRIMARY KEY,
    login TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL,
    created_at BIGINT NOT NULL
);

CREATE TABLE membership (
    user_id TEXT NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'member')),
    PRIMARY KEY (user_id, household_id)
);

CREATE TABLE invite (
    id TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'member')),
    created_by TEXT NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    expires_at BIGINT NOT NULL,
    used_at BIGINT
);

CREATE TABLE session (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    csrf_token TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    last_seen BIGINT NOT NULL,
    expires_at BIGINT NOT NULL
);

CREATE TABLE audit_event (
    id TEXT PRIMARY KEY,
    household_id TEXT,
    user_id TEXT,
    type TEXT NOT NULL,
    at BIGINT NOT NULL,
    details TEXT NOT NULL
);

CREATE INDEX idx_invite_household ON invite (household_id);

CREATE INDEX idx_session_user ON session (user_id);

CREATE INDEX idx_membership_household ON membership (household_id)
```

- [ ] **Step 7: Pomocnik testowy bazy**

`internal/store/storetest/storetest.go`:

```go
// Package storetest opens a fresh migrated database for tests.
// SQLite by default; set MEALPLAN_TEST_DRIVER=postgres and MEALPLAN_TEST_DSN for Postgres.
package storetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"mealplan/internal/store"
)

func Open(t *testing.T) *store.DB {
	t.Helper()
	var db *store.DB
	var err error
	if os.Getenv("MEALPLAN_TEST_DRIVER") == "postgres" {
		db, err = openPostgres(t)
	} else {
		dsn := "file:" + filepath.Join(t.TempDir(), "t.db") + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
		db, err = store.Open("sqlite", dsn)
	}
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func openPostgres(t *testing.T) (*store.DB, error) {
	base := os.Getenv("MEALPLAN_TEST_DSN")
	if base == "" {
		t.Skip("MEALPLAN_TEST_DSN not set")
	}
	admin, err := store.Open("postgres", base)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 6)
	_, _ = rand.Read(buf)
	schema := "t_" + hex.EncodeToString(buf)
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		_ = admin.Close()
	})
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return store.Open("postgres", u.String())
}
```

- [ ] **Step 8: Uruchom testy**

Run: `go test ./internal/store/... -v`
Expected: PASS (3 testy w `store_test`).

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat(store): database layer, migrator and initial schema"
```

---

### Task 3: Prymitywy auth (hasła, tokeny, limiter)

**Files:**
- Create: `internal/auth/password.go`, `internal/auth/token.go`, `internal/auth/limiter.go`
- Test: `internal/auth/password_test.go`, `internal/auth/token_test.go`, `internal/auth/limiter_test.go`

**Interfaces:**
- Produces:
  - `auth.Hasher{Time, Memory uint32; Threads uint8}`, `auth.DefaultHasher`, `(Hasher).Hash(pw string) (string, error)`, `auth.Verify(pw, encoded string) (bool, error)`.
  - `auth.ValidatePassword(pw string) error` (zwraca `auth.ErrPasswordLength`), stałe `auth.MinPasswordLen = 12`, `auth.MaxPasswordLen = 128`, błąd `auth.ErrInvalidHash`.
  - `auth.NewToken() (raw, hash string, err error)`, `auth.HashToken(raw string) string`, `auth.NewID() string`.
  - `auth.NewLimiter(now func() time.Time, maxFree int) *Limiter` z metodami `Allow(key string) (bool, time.Duration)`, `Fail(key string)`, `Reset(key string)`. Blokada zaczyna się na `maxFree`-tej porażce: 1 s, potem podwajana, maksymalnie 15 min. Wpis wygasa po 1 godzinie bez porażek.

- [ ] **Step 1: Dodaj zależność**

```bash
go get golang.org/x/crypto
```

- [ ] **Step 2: Napisz testy (mają nie przejść)**

`internal/auth/password_test.go`:

```go
package auth

import (
	"errors"
	"strings"
	"testing"
)

var cheap = Hasher{Time: 1, Memory: 8 * 1024, Threads: 1}

func TestHashAndVerify(t *testing.T) {
	h, err := cheap.Hash("correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Fatalf("unexpected format: %s", h)
	}
	if ok, err := Verify("correct-horse-battery", h); err != nil || !ok {
		t.Fatalf("expected match, ok=%v err=%v", ok, err)
	}
	if ok, _ := Verify("wrong-password-123", h); ok {
		t.Fatal("wrong password must not verify")
	}
}

func TestHashUsesRandomSalt(t *testing.T) {
	a, _ := cheap.Hash("same-password-123")
	b, _ := cheap.Hash("same-password-123")
	if a == b {
		t.Fatal("hashes must differ")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	for _, enc := range []string{"", "plain", "$argon2id$v=19$m=1,t=1,p=1$abc", "$bcrypt$v=19$m=1,t=1,p=1$YQ$YQ"} {
		if _, err := Verify("x", enc); !errors.Is(err, ErrInvalidHash) {
			t.Errorf("%q: expected ErrInvalidHash, got %v", enc, err)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		pw string
		ok bool
	}{
		{strings.Repeat("a", 11), false},
		{strings.Repeat("a", 12), true},
		{strings.Repeat("a", 128), true},
		{strings.Repeat("a", 129), false},
		{strings.Repeat("ą", 12), true}, // counted in runes, not bytes
		{strings.Repeat("ą", 129), false},
	}
	for _, c := range cases {
		err := ValidatePassword(c.pw)
		if c.ok && err != nil || !c.ok && !errors.Is(err, ErrPasswordLength) {
			t.Errorf("len=%d ok=%v err=%v", len([]rune(c.pw)), c.ok, err)
		}
	}
}
```

`internal/auth/token_test.go`:

```go
package auth

import "testing"

func TestNewTokenAndHash(t *testing.T) {
	raw, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 43 { // 32 bytes, base64url without padding
		t.Fatalf("raw length %d", len(raw))
	}
	if hash != HashToken(raw) || len(hash) != 64 {
		t.Fatalf("hash mismatch: %s", hash)
	}
	raw2, _, _ := NewToken()
	if raw == raw2 {
		t.Fatal("tokens must be unique")
	}
}

func TestNewID(t *testing.T) {
	a, b := NewID(), NewID()
	if len(a) != 32 || a == b {
		t.Fatalf("bad ids: %s %s", a, b)
	}
}
```

`internal/auth/limiter_test.go`:

```go
package auth

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)} }

func TestLimiterLocksOnThresholdAndDoubles(t *testing.T) {
	c := newClock()
	l := NewLimiter(c.Now, 5)
	for i := 0; i < 4; i++ {
		l.Fail("k")
		if ok, _ := l.Allow("k"); !ok {
			t.Fatalf("locked too early after %d failures", i+1)
		}
	}
	l.Fail("k") // 5th failure
	ok, wait := l.Allow("k")
	if ok || wait != time.Second {
		t.Fatalf("expected 1s lock, ok=%v wait=%v", ok, wait)
	}
	c.Advance(time.Second)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("lock should expire")
	}
	l.Fail("k") // 6th failure
	if _, wait := l.Allow("k"); wait != 2*time.Second {
		t.Fatalf("expected 2s lock, got %v", wait)
	}
}

func TestLimiterLockoutIsCapped(t *testing.T) {
	c := newClock()
	l := NewLimiter(c.Now, 5)
	for i := 0; i < 60; i++ {
		l.Fail("k")
		if _, wait := l.Allow("k"); wait > 15*time.Minute {
			t.Fatalf("lock exceeds cap: %v", wait)
		}
		c.Advance(time.Minute)
	}
}

func TestLimiterResetAndForget(t *testing.T) {
	c := newClock()
	l := NewLimiter(c.Now, 2)
	l.Fail("a")
	l.Fail("a")
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("expected lock")
	}
	l.Reset("a")
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("reset must unlock")
	}
	l.Fail("b")
	l.Fail("b")
	c.Advance(61 * time.Minute)
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("stale entries must be forgotten")
	}
	l.Fail("b") // counter starts from zero again
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("single failure after forgetting must not lock")
	}
}

func TestLimiterKeysAreIndependent(t *testing.T) {
	c := newClock()
	l := NewLimiter(c.Now, 1)
	l.Fail("a")
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("other key must be unaffected")
	}
}
```

- [ ] **Step 3: Uruchom testy i potwierdź, że nie przechodzą**

Run: `go test ./internal/auth/ -v`
Expected: FAIL (`undefined: Hasher`).

- [ ] **Step 4: Zaimplementuj `password.go`**

```go
// Package auth implements passwords, sessions, invites and login throttling.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	MinPasswordLen = 12
	MaxPasswordLen = 128
	keyLen         = 32
)

var (
	ErrInvalidHash    = errors.New("auth: invalid password hash")
	ErrPasswordLength = errors.New("auth: password must be 12 to 128 characters")
)

type Hasher struct {
	Time    uint32
	Memory  uint32 // KiB
	Threads uint8
}

var DefaultHasher = Hasher{Time: 3, Memory: 64 * 1024, Threads: 2}

var b64 = base64.RawStdEncoding

func (h Hasher) Hash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, h.Time, h.Memory, h.Threads, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.Memory, h.Time, h.Threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

func Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrInvalidHash
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, ErrInvalidHash
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, ErrInvalidHash
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func ValidatePassword(pw string) error {
	n := utf8.RuneCountInString(pw)
	if n < MinPasswordLen || n > MaxPasswordLen {
		return ErrPasswordLength
	}
	return nil
}
```

- [ ] **Step 5: Zaimplementuj `token.go`**

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// NewToken returns a 256-bit random token and its SHA-256 hash (hex). Only the hash is stored.
func NewToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// NewID returns a random 128-bit identifier as 32 hex characters.
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	return hex.EncodeToString(b)
}
```

- [ ] **Step 6: Zaimplementuj `limiter.go`**

```go
package auth

import (
	"sync"
	"time"
)

const limiterForget = time.Hour

// Limiter throttles repeated failures per key. State is in memory (per process).
type Limiter struct {
	mu      sync.Mutex
	now     func() time.Time
	maxFree int
	base    time.Duration
	max     time.Duration
	entries map[string]*limiterEntry
}

type limiterEntry struct {
	fails       int
	lockedUntil time.Time
	lastFail    time.Time
}

// NewLimiter locks a key from its maxFree-th consecutive failure on:
// 1s, doubling with each further failure, capped at 15 minutes.
func NewLimiter(now func() time.Time, maxFree int) *Limiter {
	return &Limiter{now: now, maxFree: maxFree, base: time.Second, max: 15 * time.Minute, entries: map[string]*limiterEntry{}}
}

func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	if e == nil {
		return true, 0
	}
	now := l.now()
	if now.Sub(e.lastFail) > limiterForget {
		delete(l.entries, key)
		return true, 0
	}
	if now.Before(e.lockedUntil) {
		return false, e.lockedUntil.Sub(now)
	}
	return true, 0
}

func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e := l.entries[key]
	if e == nil {
		e = &limiterEntry{}
		l.entries[key] = e
	}
	if e.fails > 0 && now.Sub(e.lastFail) > limiterForget {
		e.fails = 0
	}
	e.fails++
	e.lastFail = now
	if e.fails >= l.maxFree {
		shift := e.fails - l.maxFree
		if shift > 20 {
			shift = 20
		}
		d := l.base << uint(shift)
		if d > l.max {
			d = l.max
		}
		e.lockedUntil = now.Add(d)
	}
	if len(l.entries) > 10000 {
		for k, v := range l.entries {
			if now.Sub(v.lastFail) > limiterForget {
				delete(l.entries, k)
			}
		}
	}
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}
```

- [ ] **Step 7: Uruchom testy**

Run: `go test ./internal/auth/ -v`
Expected: PASS (wszystkie testy z `password_test`, `token_test`, `limiter_test`).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(auth): argon2id hashing, tokens and login limiter"
```

---

### Task 4: Typy domenowe i repozytoria

**Files:**
- Create: `internal/domain/domain.go`
- Create: `internal/store/store.go`, `internal/store/users.go`, `internal/store/invites.go`, `internal/store/sessions.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: `store.DB`, `storetest.Open` (Task 2).
- Produces:
  - `domain.Role` (`RoleAdmin`, `RoleMember`, `Valid() bool`), `domain.Household{ID, Name string; CreatedAt time.Time}`, `domain.User{ID, Login, PasswordHash, DisplayName string; CreatedAt time.Time}`, `domain.Member{User domain.User; Role domain.Role}`, `domain.Invite{ID, HouseholdID, TokenHash string; Role Role; CreatedBy string; ExpiresAt time.Time; UsedAt *time.Time}`, `domain.Session{ID, UserID, HouseholdID, TokenHash, CSRFToken string; CreatedAt, LastSeen, ExpiresAt time.Time}`, `domain.AuditEvent{ID, HouseholdID, UserID, Type, Details string; At time.Time}`.
  - `store.New(*DB) *Store`; błędy `ErrNotFound`, `ErrConflict`, `ErrSetupDone`, `ErrInviteUnusable`.
  - Metody `*Store`:
    - `CountUsers(ctx) (int, error)`
    - `CreateHouseholdWithAdmin(ctx, h domain.Household, u domain.User, onlyIfNoUsers bool) error`
    - `UserByLogin(ctx, login string) (domain.User, error)`, `UserByID(ctx, id string) (domain.User, error)`, `HouseholdByID(ctx, id string) (domain.Household, error)`
    - `FirstMembership(ctx, userID string) (householdID string, role domain.Role, err error)`, `RoleOf(ctx, userID, householdID string) (domain.Role, error)`, `ListMembers(ctx, householdID string) ([]domain.Member, error)`
    - `CreateInvite(ctx, inv domain.Invite) error`, `InviteByTokenHash(ctx, hash string) (domain.Invite, error)`, `ListPendingInvites(ctx, householdID string, now time.Time) ([]domain.Invite, error)`, `RevokeInvite(ctx, householdID, id string) error`, `AcceptInvite(ctx, inviteID string, u domain.User, now time.Time) (householdID string, err error)`
    - `CreateSession(ctx, s domain.Session) error`, `SessionByTokenHash(ctx, hash string) (domain.Session, error)`, `TouchSession(ctx, id string, lastSeen time.Time) error`, `DeleteSession(ctx, id string) error`
    - `InsertAudit(ctx, e domain.AuditEvent) error`

- [ ] **Step 1: Typy domenowe**

`internal/domain/domain.go`:

```go
// Package domain holds the core types shared across packages.
package domain

import "time"

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

func (r Role) Valid() bool { return r == RoleAdmin || r == RoleMember }

type Household struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type User struct {
	ID           string
	Login        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
}

type Member struct {
	User User
	Role Role
}

type Invite struct {
	ID          string
	HouseholdID string
	TokenHash   string
	Role        Role
	CreatedBy   string
	ExpiresAt   time.Time
	UsedAt      *time.Time
}

type Session struct {
	ID          string
	UserID      string
	HouseholdID string
	TokenHash   string
	CSRFToken   string
	CreatedAt   time.Time
	LastSeen    time.Time
	ExpiresAt   time.Time
}

type AuditEvent struct {
	ID          string
	HouseholdID string
	UserID      string
	Type        string
	Details     string
	At          time.Time
}
```

- [ ] **Step 2: Napisz testy repozytoriów (mają nie przejść)**

`internal/store/store_test.go`:

```go
package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

var now = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	return store.New(storetest.Open(t))
}

func user(id, login string) domain.User {
	return domain.User{ID: id, Login: login, PasswordHash: "h", DisplayName: login, CreatedAt: now}
}

func household(id string) domain.Household {
	return domain.Household{ID: id, Name: "Dom " + id, CreatedAt: now}
}

func TestSetupOnlyWhenNoUsers(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if err := s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), true); err != nil {
		t.Fatal(err)
	}
	err := s.CreateHouseholdWithAdmin(ctx, household("h2"), user("u2", "eve"), true)
	if !errors.Is(err, store.ErrSetupDone) {
		t.Fatalf("expected ErrSetupDone, got %v", err)
	}
	if n, _ := s.CountUsers(ctx); n != 1 {
		t.Fatalf("users=%d", n)
	}
	hid, role, err := s.FirstMembership(ctx, "u1")
	if err != nil || hid != "h1" || role != domain.RoleAdmin {
		t.Fatalf("membership: %s %s %v", hid, role, err)
	}
}

func TestDuplicateLoginIsConflict(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if err := s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), false); err != nil {
		t.Fatal(err)
	}
	err := s.CreateHouseholdWithAdmin(ctx, household("h2"), user("u2", "marc"), false)
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestUserLookups(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), false)
	if u, err := s.UserByLogin(ctx, "marc"); err != nil || u.ID != "u1" {
		t.Fatalf("by login: %+v %v", u, err)
	}
	if _, err := s.UserByLogin(ctx, "nobody"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if h, err := s.HouseholdByID(ctx, "h1"); err != nil || h.Name != "Dom h1" {
		t.Fatalf("household: %+v %v", h, err)
	}
}

func inviteFor(id, hid, hash string, ttl time.Duration) domain.Invite {
	return domain.Invite{ID: id, HouseholdID: hid, TokenHash: hash, Role: domain.RoleMember, CreatedBy: "u1", ExpiresAt: now.Add(ttl)}
}

func TestAcceptInviteIsSingleUse(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), false)
	if err := s.CreateInvite(ctx, inviteFor("i1", "h1", "hash1", time.Hour)); err != nil {
		t.Fatal(err)
	}
	hid, err := s.AcceptInvite(ctx, "i1", user("u2", "anna"), now)
	if err != nil || hid != "h1" {
		t.Fatalf("accept: %s %v", hid, err)
	}
	role, err := s.RoleOf(ctx, "u2", "h1")
	if err != nil || role != domain.RoleMember {
		t.Fatalf("role: %s %v", role, err)
	}
	if _, err := s.AcceptInvite(ctx, "i1", user("u3", "ola"), now); !errors.Is(err, store.ErrInviteUnusable) {
		t.Fatalf("second accept: %v", err)
	}
}

func TestAcceptInviteRejectsExpired(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), false)
	_ = s.CreateInvite(ctx, inviteFor("i1", "h1", "hash1", -time.Minute))
	if _, err := s.AcceptInvite(ctx, "i1", user("u2", "anna"), now); !errors.Is(err, store.ErrInviteUnusable) {
		t.Fatalf("expected ErrInviteUnusable, got %v", err)
	}
}

func TestAcceptInviteConcurrentOnlyOneWins(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), false)
	_ = s.CreateInvite(ctx, inviteFor("i1", "h1", "hash1", time.Hour))
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i, login := range []string{"anna", "ola"} {
		wg.Add(1)
		go func(i int, login string) {
			defer wg.Done()
			_, results[i] = s.AcceptInvite(ctx, "i1", user("id-"+login, login), now)
		}(i, login)
	}
	wg.Wait()
	wins := 0
	for _, err := range results {
		if err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("expected exactly one winner, results=%v", results)
	}
	if members, _ := s.ListMembers(ctx, "h1"); len(members) != 2 {
		t.Fatalf("members=%d", len(members))
	}
}

func TestInvitesAreScopedToHousehold(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("hA"), user("uA", "alice"), false)
	_ = s.CreateHouseholdWithAdmin(ctx, household("hB"), user("uB", "bob"), false)
	_ = s.CreateInvite(ctx, domain.Invite{ID: "iA", HouseholdID: "hA", TokenHash: "tA", Role: domain.RoleMember, CreatedBy: "uA", ExpiresAt: now.Add(time.Hour)})
	if err := s.RevokeInvite(ctx, "hB", "iA"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("household B must not revoke A's invite: %v", err)
	}
	if list, _ := s.ListPendingInvites(ctx, "hB", now); len(list) != 0 {
		t.Fatalf("B sees %d invites", len(list))
	}
	if list, _ := s.ListPendingInvites(ctx, "hA", now); len(list) != 1 {
		t.Fatalf("A sees %d invites", len(list))
	}
	if err := s.RevokeInvite(ctx, "hA", "iA"); err != nil {
		t.Fatal(err)
	}
}

func TestMembersAreScopedToHousehold(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("hA"), user("uA", "alice"), false)
	_ = s.CreateHouseholdWithAdmin(ctx, household("hB"), user("uB", "bob"), false)
	members, err := s.ListMembers(ctx, "hB")
	if err != nil || len(members) != 1 || members[0].User.Login != "bob" || members[0].Role != domain.RoleAdmin {
		t.Fatalf("members: %+v %v", members, err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_ = s.CreateHouseholdWithAdmin(ctx, household("h1"), user("u1", "marc"), false)
	sess := domain.Session{ID: "s1", UserID: "u1", HouseholdID: "h1", TokenHash: "th", CSRFToken: "c", CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(time.Hour)}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	got, err := s.SessionByTokenHash(ctx, "th")
	if err != nil || got.ID != "s1" || got.CSRFToken != "c" || !got.ExpiresAt.Equal(sess.ExpiresAt) {
		t.Fatalf("lookup: %+v %v", got, err)
	}
	later := now.Add(10 * time.Minute)
	if err := s.TouchSession(ctx, "s1", later); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.SessionByTokenHash(ctx, "th"); !got.LastSeen.Equal(later) {
		t.Fatalf("last_seen not updated: %v", got.LastSeen)
	}
	if err := s.DeleteSession(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionByTokenHash(ctx, "th"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAuditInsert(t *testing.T) {
	s := newStore(t)
	err := s.InsertAudit(context.Background(), domain.AuditEvent{ID: "a1", Type: "login_failed", At: now, Details: "ip=1.2.3.4"})
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 3: Uruchom testy i potwierdź, że nie przechodzą**

Run: `go test ./internal/store/ -v`
Expected: FAIL (`undefined: store.New`).

- [ ] **Step 4: `store.go` (błędy i helpery)**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound       = errors.New("store: not found")
	ErrConflict       = errors.New("store: conflict")
	ErrSetupDone      = errors.New("store: setup already done")
	ErrInviteUnusable = errors.New("store: invite unusable")
)

type Store struct{ db *DB }

func New(db *DB) *Store { return &Store{db: db} }

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type scanner interface{ Scan(dest ...any) error }

func (s *Store) exec(ctx context.Context, e execer, q string, args ...any) error {
	_, err := e.ExecContext(ctx, s.db.Rebind(q), args...)
	return mapErr(err)
}

func (s *Store) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unique constraint") || strings.Contains(msg, "duplicate key") {
		return ErrConflict
	}
	return err
}

func ts(t time.Time) int64 { return t.Unix() }

func fromTS(n int64) time.Time { return time.Unix(n, 0).UTC() }
```

- [ ] **Step 5: `users.go`**

```go
package store

import (
	"context"
	"database/sql"

	"mealplan/internal/domain"
)

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_user`).Scan(&n)
	return n, err
}

// CreateHouseholdWithAdmin creates a household and its first admin atomically.
// With onlyIfNoUsers it fails with ErrSetupDone when any user already exists.
func (s *Store) CreateHouseholdWithAdmin(ctx context.Context, h domain.Household, u domain.User, onlyIfNoUsers bool) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if onlyIfNoUsers {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_user`).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return ErrSetupDone
			}
		}
		if err := s.exec(ctx, tx, `INSERT INTO household (id, name, created_at) VALUES (?, ?, ?)`,
			h.ID, h.Name, ts(h.CreatedAt)); err != nil {
			return err
		}
		if err := s.insertUser(ctx, tx, u); err != nil {
			return err
		}
		return s.exec(ctx, tx, `INSERT INTO membership (user_id, household_id, role) VALUES (?, ?, ?)`,
			u.ID, h.ID, string(domain.RoleAdmin))
	})
}

func (s *Store) insertUser(ctx context.Context, e execer, u domain.User) error {
	return s.exec(ctx, e, `INSERT INTO app_user (id, login, password_hash, display_name, created_at) VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Login, u.PasswordHash, u.DisplayName, ts(u.CreatedAt))
}

func scanUser(r scanner) (domain.User, error) {
	var u domain.User
	var created int64
	if err := r.Scan(&u.ID, &u.Login, &u.PasswordHash, &u.DisplayName, &created); err != nil {
		return domain.User{}, mapErr(err)
	}
	u.CreatedAt = fromTS(created)
	return u, nil
}

const userCols = `id, login, password_hash, display_name, created_at`

func (s *Store) UserByLogin(ctx context.Context, login string) (domain.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+userCols+` FROM app_user WHERE login = ?`), login))
}

func (s *Store) UserByID(ctx context.Context, id string) (domain.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+userCols+` FROM app_user WHERE id = ?`), id))
}

func (s *Store) HouseholdByID(ctx context.Context, id string) (domain.Household, error) {
	var h domain.Household
	var created int64
	err := s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT id, name, created_at FROM household WHERE id = ?`), id).
		Scan(&h.ID, &h.Name, &created)
	if err != nil {
		return domain.Household{}, mapErr(err)
	}
	h.CreatedAt = fromTS(created)
	return h, nil
}

func (s *Store) FirstMembership(ctx context.Context, userID string) (string, domain.Role, error) {
	var hid, role string
	err := s.db.QueryRowContext(ctx, s.db.Rebind(
		`SELECT household_id, role FROM membership WHERE user_id = ? ORDER BY household_id LIMIT 1`), userID).Scan(&hid, &role)
	if err != nil {
		return "", "", mapErr(err)
	}
	return hid, domain.Role(role), nil
}

func (s *Store) RoleOf(ctx context.Context, userID, householdID string) (domain.Role, error) {
	var role string
	err := s.db.QueryRowContext(ctx, s.db.Rebind(
		`SELECT role FROM membership WHERE user_id = ? AND household_id = ?`), userID, householdID).Scan(&role)
	if err != nil {
		return "", mapErr(err)
	}
	return domain.Role(role), nil
}

func (s *Store) ListMembers(ctx context.Context, householdID string) ([]domain.Member, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(
		`SELECT u.id, u.login, u.password_hash, u.display_name, u.created_at, m.role
		   FROM membership m JOIN app_user u ON u.id = m.user_id
		  WHERE m.household_id = ? ORDER BY u.display_name, u.id`), householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Member
	for rows.Next() {
		var m domain.Member
		var created int64
		var role string
		if err := rows.Scan(&m.User.ID, &m.User.Login, &m.User.PasswordHash, &m.User.DisplayName, &created, &role); err != nil {
			return nil, err
		}
		m.User.CreatedAt = fromTS(created)
		m.Role = domain.Role(role)
		out = append(out, m)
	}
	return out, rows.Err()
}
```

- [ ] **Step 6: `invites.go`**

```go
package store

import (
	"context"
	"database/sql"
	"time"

	"mealplan/internal/domain"
)

func (s *Store) CreateInvite(ctx context.Context, inv domain.Invite) error {
	return s.exec(ctx, s.db, `INSERT INTO invite (id, household_id, token_hash, role, created_by, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		inv.ID, inv.HouseholdID, inv.TokenHash, string(inv.Role), inv.CreatedBy, ts(inv.ExpiresAt))
}

const inviteCols = `id, household_id, token_hash, role, created_by, expires_at, used_at`

func scanInvite(r scanner) (domain.Invite, error) {
	var inv domain.Invite
	var role string
	var expires int64
	var used sql.NullInt64
	if err := r.Scan(&inv.ID, &inv.HouseholdID, &inv.TokenHash, &role, &inv.CreatedBy, &expires, &used); err != nil {
		return domain.Invite{}, mapErr(err)
	}
	inv.Role = domain.Role(role)
	inv.ExpiresAt = fromTS(expires)
	if used.Valid {
		t := fromTS(used.Int64)
		inv.UsedAt = &t
	}
	return inv, nil
}

func (s *Store) InviteByTokenHash(ctx context.Context, hash string) (domain.Invite, error) {
	return scanInvite(s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT `+inviteCols+` FROM invite WHERE token_hash = ?`), hash))
}

func (s *Store) ListPendingInvites(ctx context.Context, householdID string, now time.Time) ([]domain.Invite, error) {
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(
		`SELECT `+inviteCols+` FROM invite WHERE household_id = ? AND used_at IS NULL AND expires_at > ? ORDER BY expires_at, id`),
		householdID, ts(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Invite
	for rows.Next() {
		inv, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// RevokeInvite deletes an unused invite, scoped to the household.
func (s *Store) RevokeInvite(ctx context.Context, householdID, id string) error {
	res, err := s.db.ExecContext(ctx, s.db.Rebind(
		`DELETE FROM invite WHERE id = ? AND household_id = ? AND used_at IS NULL`), id, householdID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// AcceptInvite atomically consumes an invite and creates the user and membership.
// It fails with ErrInviteUnusable if the invite is used or expired, and ErrConflict if the login is taken.
func (s *Store) AcceptInvite(ctx context.Context, inviteID string, u domain.User, now time.Time) (string, error) {
	var householdID string
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, s.db.Rebind(
			`UPDATE invite SET used_at = ? WHERE id = ? AND used_at IS NULL AND expires_at > ?`),
			ts(now), inviteID, ts(now))
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrInviteUnusable
		}
		var role string
		if err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT household_id, role FROM invite WHERE id = ?`), inviteID).
			Scan(&householdID, &role); err != nil {
			return mapErr(err)
		}
		if err := s.insertUser(ctx, tx, u); err != nil {
			return err
		}
		return s.exec(ctx, tx, `INSERT INTO membership (user_id, household_id, role) VALUES (?, ?, ?)`, u.ID, householdID, role)
	})
	if err != nil {
		return "", err
	}
	return householdID, nil
}
```

- [ ] **Step 7: `sessions.go`**

```go
package store

import (
	"context"
	"time"

	"mealplan/internal/domain"
)

func (s *Store) CreateSession(ctx context.Context, x domain.Session) error {
	return s.exec(ctx, s.db, `INSERT INTO session (id, user_id, household_id, token_hash, csrf_token, created_at, last_seen, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		x.ID, x.UserID, x.HouseholdID, x.TokenHash, x.CSRFToken, ts(x.CreatedAt), ts(x.LastSeen), ts(x.ExpiresAt))
}

func (s *Store) SessionByTokenHash(ctx context.Context, hash string) (domain.Session, error) {
	var x domain.Session
	var created, seen, expires int64
	err := s.db.QueryRowContext(ctx, s.db.Rebind(
		`SELECT id, user_id, household_id, token_hash, csrf_token, created_at, last_seen, expires_at FROM session WHERE token_hash = ?`), hash).
		Scan(&x.ID, &x.UserID, &x.HouseholdID, &x.TokenHash, &x.CSRFToken, &created, &seen, &expires)
	if err != nil {
		return domain.Session{}, mapErr(err)
	}
	x.CreatedAt, x.LastSeen, x.ExpiresAt = fromTS(created), fromTS(seen), fromTS(expires)
	return x, nil
}

func (s *Store) TouchSession(ctx context.Context, id string, lastSeen time.Time) error {
	return s.exec(ctx, s.db, `UPDATE session SET last_seen = ? WHERE id = ?`, ts(lastSeen), id)
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	return s.exec(ctx, s.db, `DELETE FROM session WHERE id = ?`, id)
}

func (s *Store) InsertAudit(ctx context.Context, e domain.AuditEvent) error {
	var hid, uid any
	if e.HouseholdID != "" {
		hid = e.HouseholdID
	}
	if e.UserID != "" {
		uid = e.UserID
	}
	return s.exec(ctx, s.db, `INSERT INTO audit_event (id, household_id, user_id, type, at, details) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID, hid, uid, e.Type, ts(e.At), e.Details)
}
```

- [ ] **Step 8: Uruchom testy**

Run: `go vet ./... && go test ./internal/store/... -race -v`
Expected: PASS, w tym `TestAcceptInviteConcurrentOnlyOneWins`.

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat(store): domain types and household-scoped repositories"
```

---

### Task 5: Serwis auth (logowanie, sesje, zaproszenia)

**Files:**
- Create: `internal/auth/service.go`
- Test: `internal/auth/service_test.go`

**Interfaces:**
- Consumes: `store.Store` (Task 4), `Hasher`, `Verify`, `ValidatePassword`, `NewToken`, `HashToken`, `NewID`, `NewLimiter` (Task 3).
- Produces:
  - `auth.Config{SessionIdle, SessionMax, InviteTTL time.Duration; Hasher Hasher}`, `auth.NewService(st *store.Store, cfg Config, now func() time.Time) (*Service, error)` (`now == nil` oznacza `time.Now`).
  - Typy: `SessionGrant{Token, CSRFToken string; ExpiresAt time.Time}`, `Principal{UserID, Login, DisplayName, HouseholdID, HouseholdName, SessionID, CSRFToken string; Role domain.Role}`, `SetupInput{HouseholdName, Login, Password, DisplayName string}`, `AcceptInput{Login, Password, DisplayName string}`, `InviteInfo{HouseholdName string; Role domain.Role; ExpiresAt time.Time}`.
  - Błędy: `ErrInvalidCredentials`, `ErrUnauthenticated`, `ErrForbidden`, `ErrSetupDone`, `ErrInviteInvalid`, `ErrLoginTaken`, `ErrInvalidInput`, `*RateLimitedError{RetryAfter time.Duration}` (oraz `ErrPasswordLength` z Task 3).
  - Metody: `SetupRequired(ctx) (bool, error)`, `Setup(ctx, SetupInput) (SessionGrant, error)`, `Login(ctx, login, password, ip string) (SessionGrant, error)`, `Logout(ctx, rawToken string) error`, `Authenticate(ctx, rawToken string) (Principal, error)`, `CreateInvite(ctx, p Principal, role domain.Role) (rawToken string, inv domain.Invite, err error)`, `ListInvites(ctx, p Principal) ([]domain.Invite, error)`, `RevokeInvite(ctx, p Principal, id string) error` (zwraca `store.ErrNotFound`, gdy zaproszenie nie należy do gospodarstwa), `InviteInfo(ctx, rawToken string) (InviteInfo, error)`, `AcceptInvite(ctx, rawToken string, in AcceptInput) (SessionGrant, error)`, `Members(ctx, p Principal) ([]domain.Member, error)`.

- [ ] **Step 1: Napisz testy (mają nie przejść)**

`internal/auth/service_test.go`:

```go
package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"mealplan/internal/domain"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

func newTestService(t *testing.T) (*Service, *store.Store, *fakeClock) {
	t.Helper()
	st := store.New(storetest.Open(t))
	clock := newClock()
	svc, err := NewService(st, Config{
		SessionIdle: time.Hour, SessionMax: 24 * time.Hour, InviteTTL: 72 * time.Hour, Hasher: cheap,
	}, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	return svc, st, clock
}

const goodPassword = "correct-horse-battery"

func setupAdmin(t *testing.T, svc *Service) SessionGrant {
	t.Helper()
	g, err := svc.Setup(context.Background(), SetupInput{HouseholdName: "Dom", Login: "marc", Password: goodPassword, DisplayName: "Marc"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	return g
}

func TestSetupCreatesAdminAndSession(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newTestService(t)
	if need, _ := svc.SetupRequired(ctx); !need {
		t.Fatal("setup should be required on empty database")
	}
	g := setupAdmin(t, svc)
	p, err := svc.Authenticate(ctx, g.Token)
	if err != nil {
		t.Fatal(err)
	}
	if p.Role != domain.RoleAdmin || p.HouseholdName != "Dom" || p.DisplayName != "Marc" || p.CSRFToken != g.CSRFToken {
		t.Fatalf("principal: %+v", p)
	}
	if need, _ := svc.SetupRequired(ctx); need {
		t.Fatal("setup must not be required afterwards")
	}
	if _, err := svc.Setup(ctx, SetupInput{HouseholdName: "X", Login: "eve", Password: goodPassword, DisplayName: "Eve"}); !errors.Is(err, ErrSetupDone) {
		t.Fatalf("second setup: %v", err)
	}
}

func TestSetupValidatesInput(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Setup(ctx, SetupInput{HouseholdName: "Dom", Login: "marc", Password: "short", DisplayName: "M"}); !errors.Is(err, ErrPasswordLength) {
		t.Fatalf("short password: %v", err)
	}
	if _, err := svc.Setup(ctx, SetupInput{HouseholdName: "Dom", Login: "a b", Password: goodPassword, DisplayName: "M"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad login: %v", err)
	}
	if _, err := svc.Setup(ctx, SetupInput{HouseholdName: " ", Login: "marc", Password: goodPassword, DisplayName: "M"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty household: %v", err)
	}
}

func TestLoginNormalizesLogin(t *testing.T) {
	svc, _, _ := newTestService(t)
	setupAdmin(t, svc)
	if _, err := svc.Login(context.Background(), "  MARC ", goodPassword, "10.0.0.1"); err != nil {
		t.Fatalf("login with spaces and capitals: %v", err)
	}
}

func TestLoginFailuresLookAlike(t *testing.T) {
	svc, _, _ := newTestService(t)
	setupAdmin(t, svc)
	ctx := context.Background()
	_, e1 := svc.Login(ctx, "marc", "wrong-password-123", "10.0.0.1")
	_, e2 := svc.Login(ctx, "nobody", "wrong-password-123", "10.0.0.2")
	if !errors.Is(e1, ErrInvalidCredentials) || !errors.Is(e2, ErrInvalidCredentials) {
		t.Fatalf("errors: %v / %v", e1, e2)
	}
}

func TestLoginHugePasswordIsRejectedCheaply(t *testing.T) {
	svc, _, _ := newTestService(t)
	setupAdmin(t, svc)
	_, err := svc.Login(context.Background(), "marc", strings.Repeat("a", 5000), "10.0.0.1")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginLockoutAndRecovery(t *testing.T) {
	svc, _, clock := newTestService(t)
	setupAdmin(t, svc)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = svc.Login(ctx, "marc", "wrong-password-123", "10.0.0.1")
	}
	_, err := svc.Login(ctx, "marc", goodPassword, "10.0.0.1")
	var rl *RateLimitedError
	if !errors.As(err, &rl) || rl.RetryAfter <= 0 {
		t.Fatalf("expected RateLimitedError, got %v", err)
	}
	clock.Advance(time.Minute)
	if _, err := svc.Login(ctx, "marc", goodPassword, "10.0.0.1"); err != nil {
		t.Fatalf("lockout must expire: %v", err)
	}
}

func TestSessionExpiry(t *testing.T) {
	svc, _, clock := newTestService(t)
	ctx := context.Background()
	g := setupAdmin(t, svc)

	clock.Advance(30 * time.Minute)
	if _, err := svc.Authenticate(ctx, g.Token); err != nil {
		t.Fatalf("active session: %v", err)
	}
	clock.Advance(40 * time.Minute) // 40 min since last activity, 70 since start: idle limit is 60
	if _, err := svc.Authenticate(ctx, g.Token); err != nil {
		t.Fatalf("activity must extend idle window: %v", err)
	}
	clock.Advance(61 * time.Minute)
	if _, err := svc.Authenticate(ctx, g.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("idle expiry: %v", err)
	}
}

func TestSessionAbsoluteExpiry(t *testing.T) {
	svc, _, clock := newTestService(t)
	ctx := context.Background()
	g := setupAdmin(t, svc)
	for i := 0; i < 25; i++ { // stay active every 59 minutes, past the 24h cap
		clock.Advance(59 * time.Minute)
		if _, err := svc.Authenticate(ctx, g.Token); err != nil {
			if !errors.Is(err, ErrUnauthenticated) {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("session survived past absolute expiry")
}

func TestLogoutInvalidatesSession(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	g := setupAdmin(t, svc)
	if err := svc.Logout(ctx, g.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, g.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
	if _, err := svc.Authenticate(ctx, ""); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("empty token: %v", err)
	}
}

func TestInviteFlow(t *testing.T) {
	svc, st, clock := newTestService(t)
	ctx := context.Background()
	admin, _ := svc.Authenticate(ctx, setupAdmin(t, svc).Token)

	raw, inv, err := svc.CreateInvite(ctx, admin, domain.RoleMember)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.InviteByTokenHash(ctx, raw); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("raw token must not be stored")
	}
	if _, err := st.InviteByTokenHash(ctx, HashToken(raw)); err != nil {
		t.Fatalf("hashed token must be stored: %v", err)
	}
	info, err := svc.InviteInfo(ctx, raw)
	if err != nil || info.HouseholdName != "Dom" || info.Role != domain.RoleMember {
		t.Fatalf("info: %+v %v", info, err)
	}

	g, err := svc.AcceptInvite(ctx, raw, AcceptInput{Login: "Anna", Password: "another-long-password", DisplayName: "Anna"})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	member, err := svc.Authenticate(ctx, g.Token)
	if err != nil || member.Role != domain.RoleMember || member.HouseholdID != admin.HouseholdID {
		t.Fatalf("member: %+v %v", member, err)
	}
	if _, _, err := svc.CreateInvite(ctx, member, domain.RoleMember); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member creating invite: %v", err)
	}
	if _, err := svc.ListInvites(ctx, member); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member listing invites: %v", err)
	}
	if _, err := svc.AcceptInvite(ctx, raw, AcceptInput{Login: "ola", Password: "another-long-password", DisplayName: "Ola"}); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("reuse: %v", err)
	}
	if _, err := svc.InviteInfo(ctx, raw); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("info after use: %v", err)
	}
	if list, _ := svc.ListInvites(ctx, admin); len(list) != 0 {
		t.Fatalf("used invite must not be pending, got %d", len(list))
	}

	raw2, _, _ := svc.CreateInvite(ctx, admin, domain.RoleMember)
	clock.Advance(73 * time.Hour)
	if _, err := svc.AcceptInvite(ctx, raw2, AcceptInput{Login: "ola", Password: "another-long-password", DisplayName: "Ola"}); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("expired: %v", err)
	}
	_ = inv
}

func TestAcceptInviteRejectsTakenLoginAndUnknownToken(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	admin, _ := svc.Authenticate(ctx, setupAdmin(t, svc).Token)
	raw, _, _ := svc.CreateInvite(ctx, admin, domain.RoleMember)
	if _, err := svc.AcceptInvite(ctx, raw, AcceptInput{Login: "marc", Password: "another-long-password", DisplayName: "X"}); !errors.Is(err, ErrLoginTaken) {
		t.Fatalf("taken login: %v", err)
	}
	if _, err := svc.AcceptInvite(ctx, "no-such-token", AcceptInput{Login: "ola", Password: "another-long-password", DisplayName: "Ola"}); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("unknown token: %v", err)
	}
	if _, err := svc.AcceptInvite(ctx, raw, AcceptInput{Login: "ola", Password: "short", DisplayName: "Ola"}); !errors.Is(err, ErrPasswordLength) {
		t.Fatalf("short password: %v", err)
	}
	if _, _, err := svc.CreateInvite(ctx, admin, domain.Role("root")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad role: %v", err)
	}
}

func TestAcceptInviteConcurrent(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	admin, _ := svc.Authenticate(ctx, setupAdmin(t, svc).Token)
	raw, _, _ := svc.CreateInvite(ctx, admin, domain.RoleMember)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, login := range []string{"anna", "ola"} {
		wg.Add(1)
		go func(i int, login string) {
			defer wg.Done()
			_, errs[i] = svc.AcceptInvite(ctx, raw, AcceptInput{Login: login, Password: "another-long-password", DisplayName: login})
		}(i, login)
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("expected exactly one success, got %v", errs)
	}
}

func TestHouseholdIsolation(t *testing.T) {
	svc, st, clock := newTestService(t)
	ctx := context.Background()
	adminA, _ := svc.Authenticate(ctx, setupAdmin(t, svc).Token)
	_, inv, err := svc.CreateInvite(ctx, adminA, domain.RoleMember)
	if err != nil {
		t.Fatal(err)
	}

	hash, _ := svc.cfg.Hasher.Hash("bobs-long-password")
	bob := domain.User{ID: NewID(), Login: "bob", PasswordHash: hash, DisplayName: "Bob", CreatedAt: clock.Now()}
	if err := st.CreateHouseholdWithAdmin(ctx, domain.Household{ID: NewID(), Name: "Inny", CreatedAt: clock.Now()}, bob, false); err != nil {
		t.Fatal(err)
	}
	g, err := svc.Login(ctx, "bob", "bobs-long-password", "10.0.0.9")
	if err != nil {
		t.Fatal(err)
	}
	adminB, _ := svc.Authenticate(ctx, g.Token)

	if err := svc.RevokeInvite(ctx, adminB, inv.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("B revoking A's invite: %v", err)
	}
	if list, _ := svc.ListInvites(ctx, adminB); len(list) != 0 {
		t.Fatalf("B sees %d of A's invites", len(list))
	}
	members, _ := svc.Members(ctx, adminB)
	if len(members) != 1 || members[0].User.Login != "bob" {
		t.Fatalf("B members: %+v", members)
	}
}
```

- [ ] **Step 2: Uruchom testy i potwierdź, że nie przechodzą**

Run: `go test ./internal/auth/ -v`
Expected: FAIL (`undefined: NewService`).

- [ ] **Step 3: Zaimplementuj serwis**

`internal/auth/service.go`:

```go
package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"mealplan/internal/domain"
	"mealplan/internal/store"
)

var (
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrUnauthenticated    = errors.New("auth: unauthenticated")
	ErrForbidden          = errors.New("auth: forbidden")
	ErrSetupDone          = errors.New("auth: setup already done")
	ErrInviteInvalid      = errors.New("auth: invite invalid")
	ErrLoginTaken         = errors.New("auth: login taken")
	ErrInvalidInput       = errors.New("auth: invalid input")
)

type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string { return "auth: rate limited" }

type Config struct {
	SessionIdle time.Duration
	SessionMax  time.Duration
	InviteTTL   time.Duration
	Hasher      Hasher
}

type Service struct {
	st           *store.Store
	cfg          Config
	now          func() time.Time
	loginLimiter *Limiter
	ipLimiter    *Limiter
	dummyHash    string // verified against when the user does not exist, to equalize timing
}

func NewService(st *store.Store, cfg Config, now func() time.Time) (*Service, error) {
	if now == nil {
		now = time.Now
	}
	dummy, err := cfg.Hasher.Hash("timing-equalization-password")
	if err != nil {
		return nil, err
	}
	return &Service{
		st: st, cfg: cfg, now: now,
		loginLimiter: NewLimiter(now, 5),
		ipLimiter:    NewLimiter(now, 30),
		dummyHash:    dummy,
	}, nil
}

type SessionGrant struct {
	Token     string
	CSRFToken string
	ExpiresAt time.Time
}

type Principal struct {
	UserID        string
	Login         string
	DisplayName   string
	HouseholdID   string
	HouseholdName string
	SessionID     string
	CSRFToken     string
	Role          domain.Role
}

type SetupInput struct{ HouseholdName, Login, Password, DisplayName string }

type AcceptInput struct{ Login, Password, DisplayName string }

type InviteInfo struct {
	HouseholdName string
	Role          domain.Role
	ExpiresAt     time.Time
}

var loginRe = regexp.MustCompile(`^[a-z0-9._-]{3,64}$`)

func normalizeLogin(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func validateProfile(login, displayName string) error {
	if !loginRe.MatchString(login) {
		return fmt.Errorf("%w: login must be 3 to 64 characters of a-z, 0-9, '.', '_' or '-'", ErrInvalidInput)
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(displayName)); n < 1 || n > 64 {
		return fmt.Errorf("%w: display name must be 1 to 64 characters", ErrInvalidInput)
	}
	return nil
}

func (s *Service) audit(ctx context.Context, householdID, userID, typ, details string) {
	_ = s.st.InsertAudit(ctx, domain.AuditEvent{
		ID: NewID(), HouseholdID: householdID, UserID: userID, Type: typ, Details: details, At: s.now(),
	})
}

func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	n, err := s.st.CountUsers(ctx)
	return n == 0, err
}

func (s *Service) Setup(ctx context.Context, in SetupInput) (SessionGrant, error) {
	login := normalizeLogin(in.Login)
	if err := validateProfile(login, in.DisplayName); err != nil {
		return SessionGrant{}, err
	}
	name := strings.TrimSpace(in.HouseholdName)
	if n := utf8.RuneCountInString(name); n < 1 || n > 100 {
		return SessionGrant{}, fmt.Errorf("%w: household name must be 1 to 100 characters", ErrInvalidInput)
	}
	if err := ValidatePassword(in.Password); err != nil {
		return SessionGrant{}, err
	}
	hash, err := s.cfg.Hasher.Hash(in.Password)
	if err != nil {
		return SessionGrant{}, err
	}
	now := s.now()
	h := domain.Household{ID: NewID(), Name: name, CreatedAt: now}
	u := domain.User{ID: NewID(), Login: login, PasswordHash: hash, DisplayName: strings.TrimSpace(in.DisplayName), CreatedAt: now}
	if err := s.st.CreateHouseholdWithAdmin(ctx, h, u, true); err != nil {
		if errors.Is(err, store.ErrSetupDone) {
			return SessionGrant{}, ErrSetupDone
		}
		return SessionGrant{}, err
	}
	s.audit(ctx, h.ID, u.ID, "setup", "")
	return s.startSession(ctx, u.ID)
}

func (s *Service) Login(ctx context.Context, login, password, ip string) (SessionGrant, error) {
	login = normalizeLogin(login)
	loginKey, ipKey := "login:"+login, "ip:"+ip
	for _, c := range []struct {
		l   *Limiter
		key string
	}{{s.loginLimiter, loginKey}, {s.ipLimiter, ipKey}} {
		if ok, wait := c.l.Allow(c.key); !ok {
			return SessionGrant{}, &RateLimitedError{RetryAfter: wait}
		}
	}

	u, err := s.st.UserByLogin(ctx, login)
	found := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return SessionGrant{}, err
	}

	ok := false
	if utf8.RuneCountInString(password) <= MaxPasswordLen { // never hash oversized input
		hash := s.dummyHash
		if found {
			hash = u.PasswordHash
		}
		match, verr := Verify(password, hash)
		ok = verr == nil && match && found
	}
	if !ok {
		s.loginLimiter.Fail(loginKey)
		s.ipLimiter.Fail(ipKey)
		s.audit(ctx, "", "", "login_failed", "ip="+ip)
		return SessionGrant{}, ErrInvalidCredentials
	}
	s.loginLimiter.Reset(loginKey)
	s.audit(ctx, "", u.ID, "login", "ip="+ip)
	return s.startSession(ctx, u.ID)
}

func (s *Service) startSession(ctx context.Context, userID string) (SessionGrant, error) {
	hid, _, err := s.st.FirstMembership(ctx, userID)
	if err != nil {
		return SessionGrant{}, err
	}
	raw, hash, err := NewToken()
	if err != nil {
		return SessionGrant{}, err
	}
	csrf, _, err := NewToken()
	if err != nil {
		return SessionGrant{}, err
	}
	now := s.now()
	sess := domain.Session{
		ID: NewID(), UserID: userID, HouseholdID: hid, TokenHash: hash, CSRFToken: csrf,
		CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(s.cfg.SessionMax),
	}
	if err := s.st.CreateSession(ctx, sess); err != nil {
		return SessionGrant{}, err
	}
	return SessionGrant{Token: raw, CSRFToken: csrf, ExpiresAt: sess.ExpiresAt}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	sess, err := s.st.SessionByTokenHash(ctx, HashToken(rawToken))
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.st.DeleteSession(ctx, sess.ID)
}

func (s *Service) Authenticate(ctx context.Context, rawToken string) (Principal, error) {
	if rawToken == "" {
		return Principal{}, ErrUnauthenticated
	}
	sess, err := s.st.SessionByTokenHash(ctx, HashToken(rawToken))
	if errors.Is(err, store.ErrNotFound) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, err
	}
	now := s.now()
	if !now.Before(sess.ExpiresAt) || !now.Before(sess.LastSeen.Add(s.cfg.SessionIdle)) {
		_ = s.st.DeleteSession(ctx, sess.ID)
		return Principal{}, ErrUnauthenticated
	}
	role, err := s.st.RoleOf(ctx, sess.UserID, sess.HouseholdID)
	if errors.Is(err, store.ErrNotFound) {
		_ = s.st.DeleteSession(ctx, sess.ID)
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, err
	}
	user, err := s.st.UserByID(ctx, sess.UserID)
	if err != nil {
		return Principal{}, err
	}
	hh, err := s.st.HouseholdByID(ctx, sess.HouseholdID)
	if err != nil {
		return Principal{}, err
	}
	if now.Sub(sess.LastSeen) > time.Minute {
		_ = s.st.TouchSession(ctx, sess.ID, now)
	}
	return Principal{
		UserID: user.ID, Login: user.Login, DisplayName: user.DisplayName, HouseholdID: hh.ID, HouseholdName: hh.Name,
		SessionID: sess.ID, CSRFToken: sess.CSRFToken, Role: role,
	}, nil
}

func requireAdmin(p Principal) error {
	if p.Role != domain.RoleAdmin {
		return ErrForbidden
	}
	return nil
}

func (s *Service) CreateInvite(ctx context.Context, p Principal, role domain.Role) (string, domain.Invite, error) {
	if err := requireAdmin(p); err != nil {
		return "", domain.Invite{}, err
	}
	if !role.Valid() {
		return "", domain.Invite{}, fmt.Errorf("%w: role must be admin or member", ErrInvalidInput)
	}
	raw, hash, err := NewToken()
	if err != nil {
		return "", domain.Invite{}, err
	}
	inv := domain.Invite{
		ID: NewID(), HouseholdID: p.HouseholdID, TokenHash: hash, Role: role,
		CreatedBy: p.UserID, ExpiresAt: s.now().Add(s.cfg.InviteTTL),
	}
	if err := s.st.CreateInvite(ctx, inv); err != nil {
		return "", domain.Invite{}, err
	}
	s.audit(ctx, p.HouseholdID, p.UserID, "invite_created", "role="+string(role))
	return raw, inv, nil
}

func (s *Service) ListInvites(ctx context.Context, p Principal) ([]domain.Invite, error) {
	if err := requireAdmin(p); err != nil {
		return nil, err
	}
	return s.st.ListPendingInvites(ctx, p.HouseholdID, s.now())
}

func (s *Service) RevokeInvite(ctx context.Context, p Principal, id string) error {
	if err := requireAdmin(p); err != nil {
		return err
	}
	if err := s.st.RevokeInvite(ctx, p.HouseholdID, id); err != nil {
		return err
	}
	s.audit(ctx, p.HouseholdID, p.UserID, "invite_revoked", "")
	return nil
}

func (s *Service) usableInvite(ctx context.Context, rawToken string) (domain.Invite, error) {
	inv, err := s.st.InviteByTokenHash(ctx, HashToken(rawToken))
	if errors.Is(err, store.ErrNotFound) {
		return domain.Invite{}, ErrInviteInvalid
	}
	if err != nil {
		return domain.Invite{}, err
	}
	if inv.UsedAt != nil || !s.now().Before(inv.ExpiresAt) {
		return domain.Invite{}, ErrInviteInvalid
	}
	return inv, nil
}

func (s *Service) InviteInfo(ctx context.Context, rawToken string) (InviteInfo, error) {
	inv, err := s.usableInvite(ctx, rawToken)
	if err != nil {
		return InviteInfo{}, err
	}
	hh, err := s.st.HouseholdByID(ctx, inv.HouseholdID)
	if err != nil {
		return InviteInfo{}, err
	}
	return InviteInfo{HouseholdName: hh.Name, Role: inv.Role, ExpiresAt: inv.ExpiresAt}, nil
}

func (s *Service) AcceptInvite(ctx context.Context, rawToken string, in AcceptInput) (SessionGrant, error) {
	login := normalizeLogin(in.Login)
	if err := validateProfile(login, in.DisplayName); err != nil {
		return SessionGrant{}, err
	}
	if err := ValidatePassword(in.Password); err != nil {
		return SessionGrant{}, err
	}
	inv, err := s.usableInvite(ctx, rawToken)
	if err != nil {
		return SessionGrant{}, err
	}
	hash, err := s.cfg.Hasher.Hash(in.Password)
	if err != nil {
		return SessionGrant{}, err
	}
	now := s.now()
	u := domain.User{ID: NewID(), Login: login, PasswordHash: hash, DisplayName: strings.TrimSpace(in.DisplayName), CreatedAt: now}
	hid, err := s.st.AcceptInvite(ctx, inv.ID, u, now)
	switch {
	case errors.Is(err, store.ErrInviteUnusable):
		return SessionGrant{}, ErrInviteInvalid
	case errors.Is(err, store.ErrConflict):
		return SessionGrant{}, ErrLoginTaken
	case err != nil:
		return SessionGrant{}, err
	}
	s.audit(ctx, hid, u.ID, "invite_accepted", "role="+string(inv.Role))
	return s.startSession(ctx, u.ID)
}

func (s *Service) Members(ctx context.Context, p Principal) ([]domain.Member, error) {
	return s.st.ListMembers(ctx, p.HouseholdID)
}
```

- [ ] **Step 4: Uruchom testy**

Run: `go vet ./... && go test ./internal/auth/... -race -v`
Expected: PASS (wszystkie testy `auth`).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(auth): service for setup, login, sessions and invites"
```

---

### Task 6: Warstwa HTTP i kontrakt OpenAPI

**Files:**
- Create: `internal/httpapi/server.go`, `internal/httpapi/middleware.go`, `internal/httpapi/handlers.go`, `internal/httpapi/errors.go`
- Create: `api/openapi.yaml`
- Modify: `docs/superpowers/specs/2026-11-01-mealplan-design.md` (nazwy tras zaproszeń, CSP)
- Test: `internal/httpapi/server_test.go`, `internal/httpapi/openapi_test.go`

**Interfaces:**
- Consumes: `auth.Service` i typy z Task 5, `config.Config` (Task 1), `store.ErrNotFound` (Task 4).
- Produces:
  - `httpapi.New(a *auth.Service, cfg config.Config, static http.Handler, log *slog.Logger) *Server`, `(*Server).Handler() http.Handler`, `(*Server).Routes() []string` (wzorce w postaci `"POST /api/v1/auth/login"`).
  - Trasy: `GET /api/v1/auth/status`, `POST /api/v1/auth/setup`, `POST /api/v1/auth/login`, `POST /api/v1/auth/logout`, `GET /api/v1/auth/me`, `POST /api/v1/auth/invites`, `GET /api/v1/auth/invites`, `DELETE /api/v1/auth/invites/{id}`, `GET /api/v1/auth/invitations/{token}`, `POST /api/v1/auth/invitations/{token}/accept`, `GET /api/v1/household/members`.
  - Ciastka: `mealplan_session` (HttpOnly) i `mealplan_csrf` (czytelne dla JS). Nagłówek CSRF: `X-CSRF-Token`.
  - Kształty JSON (używane przez frontend w Task 7):
    - `GET /auth/status` → `{"setupRequired":bool,"mode":"selfhost|hosted"}`
    - `GET /auth/me` → `{"user":{"id","login","displayName"},"household":{"id","name"},"role":"admin|member","csrfToken":"..."}`
    - `POST /auth/setup` ciało `{"householdName","login","password","displayName"}`, `POST /auth/login` ciało `{"login","password"}`, `POST /auth/invitations/{token}/accept` ciało `{"login","password","displayName"}`: odpowiedź `204` z ciastkami.
    - `POST /auth/invites` ciało `{"role"}` → `201 {"id","token","role","expiresAt","path":"/invite/<token>"}`
    - `GET /auth/invites` → `[{"id","role","expiresAt"}]`
    - `GET /auth/invitations/{token}` → `{"householdName","role","expiresAt"}`
    - `GET /household/members` → `[{"id","login","displayName","role"}]`
    - Błędy: `{"code","message"}`; kody: `bad_request`, `too_large`, `unsupported_media_type`, `unauthenticated`, `csrf`, `forbidden`, `not_found`, `invalid_credentials`, `rate_limited` (nagłówek `Retry-After`), `setup_done`, `invite_invalid`, `login_taken`, `password_length`, `invalid_input`, `internal`.

- [ ] **Step 1: Napisz testy HTTP (mają nie przejść)**

`internal/httpapi/server_test.go`:

```go
package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mealplan/internal/auth"
	"mealplan/internal/config"
	"mealplan/internal/domain"
	"mealplan/internal/httpapi"
	"mealplan/internal/store"
	"mealplan/internal/store/storetest"
)

var cheap = auth.Hasher{Time: 1, Memory: 8 * 1024, Threads: 1}

const pw = "correct-horse-battery"

type env struct {
	srv *httpapi.Server
	st  *store.Store
	svc *auth.Service
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st := store.New(storetest.Open(t))
	svc, err := auth.NewService(st, auth.Config{SessionIdle: time.Hour, SessionMax: 24 * time.Hour, InviteTTL: 72 * time.Hour, Hasher: cheap}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Mode: config.ModeSelfhost, CookieSecure: false}
	srv := httpapi.New(svc, cfg, http.NotFoundHandler(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &env{srv: srv, st: st, svc: svc}
}

type client struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]string
	csrf    string
}

func (e *env) client(t *testing.T) *client {
	return &client{t: t, h: e.srv.Handler(), cookies: map[string]string{}}
}

func (c *client) do(method, path string, body any) *httptest.ResponseRecorder {
	c.t.Helper()
	var rdr io.Reader
	if s, ok := body.(string); ok {
		rdr = strings.NewReader(s)
	} else if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range c.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		if ck.MaxAge < 0 || ck.Value == "" {
			delete(c.cookies, ck.Name)
		} else {
			c.cookies[ck.Name] = ck.Value
		}
	}
	return rec
}

func (c *client) useCSRF() { c.csrf = c.cookies["mealplan_csrf"] }

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var e struct{ Code string }
	decode(t, rec, &e)
	return e.Code
}

func setup(t *testing.T, c *client) {
	t.Helper()
	rec := c.do("POST", "/api/v1/auth/setup", map[string]string{"householdName": "Dom", "login": "marc", "password": pw, "displayName": "Marc"})
	if rec.Code != 204 {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body)
	}
	c.useCSRF()
}

func TestStatusAndSetup(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	var st struct {
		SetupRequired bool
		Mode          string
	}
	decode(t, c.do("GET", "/api/v1/auth/status", nil), &st)
	if !st.SetupRequired || st.Mode != "selfhost" {
		t.Fatalf("status: %+v", st)
	}
	setup(t, c)
	decode(t, c.do("GET", "/api/v1/auth/status", nil), &st)
	if st.SetupRequired {
		t.Fatal("setup must be done")
	}
	rec := c.do("POST", "/api/v1/auth/setup", map[string]string{"householdName": "X", "login": "eve", "password": pw, "displayName": "Eve"})
	if rec.Code != 409 || errCode(t, rec) != "setup_done" {
		t.Fatalf("second setup: %d %s", rec.Code, rec.Body)
	}
}

func TestMeRequiresSessionAndReturnsProfile(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	if rec := c.do("GET", "/api/v1/auth/me", nil); rec.Code != 401 || errCode(t, rec) != "unauthenticated" {
		t.Fatalf("anonymous me: %d %s", rec.Code, rec.Body)
	}
	setup(t, c)
	var me struct {
		User      struct{ Login, DisplayName string }
		Household struct{ Name string }
		Role      string
		CSRFToken string
	}
	rec := c.do("GET", "/api/v1/auth/me", nil)
	decode(t, rec, &me)
	if me.User.Login != "marc" || me.Household.Name != "Dom" || me.Role != "admin" || me.CSRFToken != c.csrf {
		t.Fatalf("me: %+v", me)
	}
}

func TestSessionCookieFlags(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	rec := c.do("POST", "/api/v1/auth/setup", map[string]string{"householdName": "Dom", "login": "marc", "password": pw, "displayName": "Marc"})
	flags := map[string]*http.Cookie{}
	for _, ck := range rec.Result().Cookies() {
		flags[ck.Name] = ck
	}
	s, x := flags["mealplan_session"], flags["mealplan_csrf"]
	if s == nil || x == nil {
		t.Fatalf("cookies: %v", flags)
	}
	if !s.HttpOnly || s.SameSite != http.SameSiteLaxMode || s.Path != "/" {
		t.Fatalf("session cookie flags: %+v", s)
	}
	if x.HttpOnly {
		t.Fatal("csrf cookie must be readable by JS")
	}
}

func TestLoginLogoutAndErrors(t *testing.T) {
	e := newEnv(t)
	setup(t, e.client(t))
	c := e.client(t)
	if rec := c.do("POST", "/api/v1/auth/login", map[string]string{"login": "marc", "password": "wrong-password-123"}); rec.Code != 401 || errCode(t, rec) != "invalid_credentials" {
		t.Fatalf("wrong password: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("POST", "/api/v1/auth/login", map[string]string{"login": " MARC", "password": pw}); rec.Code != 204 {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	c.useCSRF()
	if rec := c.do("POST", "/api/v1/auth/logout", nil); rec.Code != 204 {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("GET", "/api/v1/auth/me", nil); rec.Code != 401 {
		t.Fatalf("after logout: %d", rec.Code)
	}
}

func TestLoginRateLimited(t *testing.T) {
	e := newEnv(t)
	setup(t, e.client(t))
	c := e.client(t)
	for i := 0; i < 5; i++ {
		c.do("POST", "/api/v1/auth/login", map[string]string{"login": "marc", "password": "wrong-password-123"})
	}
	rec := c.do("POST", "/api/v1/auth/login", map[string]string{"login": "marc", "password": pw})
	if rec.Code != 429 || errCode(t, rec) != "rate_limited" || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("expected 429 with Retry-After, got %d %s %v", rec.Code, rec.Body, rec.Header())
	}
}

func TestCSRFRequiredOnAuthenticatedWrites(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	setup(t, c)
	c.csrf = ""
	if rec := c.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"}); rec.Code != 403 || errCode(t, rec) != "csrf" {
		t.Fatalf("missing csrf: %d %s", rec.Code, rec.Body)
	}
	c.csrf = "forged"
	if rec := c.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"}); rec.Code != 403 {
		t.Fatalf("forged csrf: %d", rec.Code)
	}
	c.useCSRF()
	if rec := c.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"}); rec.Code != 201 {
		t.Fatalf("valid csrf: %d %s", rec.Code, rec.Body)
	}
}

func TestPublicPostsRequireJSONContentType(t *testing.T) {
	e := newEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader("login=marc&password=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 415 || errCode(t, rec) != "unsupported_media_type" {
		t.Fatalf("expected 415, got %d %s", rec.Code, rec.Body)
	}
}

func TestOversizedBodyAndPassword(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	big := `{"login":"marc","password":"` + strings.Repeat("a", 2<<20) + `"}`
	if rec := c.do("POST", "/api/v1/auth/login", big); rec.Code != 413 || errCode(t, rec) != "too_large" {
		t.Fatalf("big body: %d %s", rec.Code, rec.Body)
	}
	rec := c.do("POST", "/api/v1/auth/setup", map[string]string{"householdName": "Dom", "login": "marc", "password": strings.Repeat("a", 200), "displayName": "M"})
	if rec.Code != 422 || errCode(t, rec) != "password_length" {
		t.Fatalf("long password: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do("POST", "/api/v1/auth/login", `{not json`); rec.Code != 400 || errCode(t, rec) != "bad_request" {
		t.Fatalf("bad json: %d %s", rec.Code, rec.Body)
	}
}

func TestInviteEndToEnd(t *testing.T) {
	e := newEnv(t)
	admin := e.client(t)
	setup(t, admin)

	rec := admin.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"})
	var created struct{ ID, Token, Role, Path string }
	decode(t, rec, &created)
	if rec.Code != 201 || created.Token == "" || created.Path != "/invite/"+created.Token {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var list []struct{ ID, Role string }
	decode(t, admin.do("GET", "/api/v1/auth/invites", nil), &list)
	if len(list) != 1 || list[0].ID != created.ID || strings.Contains(admin.do("GET", "/api/v1/auth/invites", nil).Body.String(), created.Token) {
		t.Fatalf("list must show the invite without its token: %+v", list)
	}

	guest := e.client(t)
	var info struct{ HouseholdName, Role string }
	decode(t, guest.do("GET", "/api/v1/auth/invitations/"+created.Token, nil), &info)
	if info.HouseholdName != "Dom" || info.Role != "member" {
		t.Fatalf("info: %+v", info)
	}
	if rec := guest.do("GET", "/api/v1/auth/invitations/bogus", nil); rec.Code != 404 || errCode(t, rec) != "invite_invalid" {
		t.Fatalf("bogus token: %d %s", rec.Code, rec.Body)
	}
	rec = guest.do("POST", "/api/v1/auth/invitations/"+created.Token+"/accept", map[string]string{"login": "anna", "password": "another-long-password", "displayName": "Anna"})
	if rec.Code != 204 {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body)
	}
	guest.useCSRF()
	var me struct{ Role string }
	decode(t, guest.do("GET", "/api/v1/auth/me", nil), &me)
	if me.Role != "member" {
		t.Fatalf("role: %s", me.Role)
	}
	if rec := guest.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"}); rec.Code != 403 || errCode(t, rec) != "forbidden" {
		t.Fatalf("member creating invite: %d %s", rec.Code, rec.Body)
	}
	var members []struct{ Login, Role string }
	decode(t, guest.do("GET", "/api/v1/household/members", nil), &members)
	if len(members) != 2 {
		t.Fatalf("members: %+v", members)
	}
	if strings.Contains(guest.do("GET", "/api/v1/household/members", nil).Body.String(), "argon2") {
		t.Fatal("password hash leaked")
	}
	if rec := guest.do("POST", "/api/v1/auth/invitations/"+created.Token+"/accept", map[string]string{"login": "ola", "password": "another-long-password", "displayName": "Ola"}); rec.Code != 404 {
		t.Fatalf("reused invite: %d", rec.Code)
	}
}

func TestHouseholdIsolationOverHTTP(t *testing.T) {
	e := newEnv(t)
	a := e.client(t)
	setup(t, a)
	var created struct{ ID string }
	decode(t, a.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"}), &created)

	hash, _ := cheap.Hash("bobs-long-password")
	bob := domain.User{ID: auth.NewID(), Login: "bob", PasswordHash: hash, DisplayName: "Bob", CreatedAt: time.Now()}
	if err := e.st.CreateHouseholdWithAdmin(context.Background(), domain.Household{ID: auth.NewID(), Name: "Inny", CreatedAt: time.Now()}, bob, false); err != nil {
		t.Fatal(err)
	}
	b := e.client(t)
	if rec := b.do("POST", "/api/v1/auth/login", map[string]string{"login": "bob", "password": "bobs-long-password"}); rec.Code != 204 {
		t.Fatalf("bob login: %d %s", rec.Code, rec.Body)
	}
	b.useCSRF()
	if rec := b.do("DELETE", "/api/v1/auth/invites/"+created.ID, nil); rec.Code != 404 {
		t.Fatalf("B deleting A's invite: %d", rec.Code)
	}
	var members []struct{ Login string }
	decode(t, b.do("GET", "/api/v1/household/members", nil), &members)
	if len(members) != 1 || members[0].Login != "bob" {
		t.Fatalf("B members: %+v", members)
	}
	if rec := a.do("DELETE", "/api/v1/auth/invites/"+created.ID, nil); rec.Code != 204 {
		t.Fatalf("A deleting own invite: %d %s", rec.Code, rec.Body)
	}
}

func TestSecurityHeadersAndUnknownAPIPath(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	rec := c.do("GET", "/api/v1/auth/status", nil)
	for h, want := range map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"X-Frame-Options":         "DENY",
		"Cache-Control":           "no-store",
	} {
		if got := rec.Header().Get(h); got != want {
			t.Errorf("%s = %q, want %q", h, got, want)
		}
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("csp: %s", csp)
	}
	rec = c.do("GET", "/api/v1/nope", nil)
	if rec.Code != 404 || errCode(t, rec) != "not_found" {
		t.Fatalf("unknown api path: %d %s", rec.Code, rec.Body)
	}
}
```

`internal/httpapi/openapi_test.go`:

```go
package httpapi_test

import (
	"os"
	"strings"
	"testing"
)

// The OpenAPI file must document exactly the routes the server registers.
func TestRoutesMatchOpenAPI(t *testing.T) {
	raw, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true}
	documented := map[string]bool{}
	path := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "  /") && strings.HasSuffix(line, ":") {
			path = strings.TrimSuffix(strings.TrimSpace(line), ":")
			continue
		}
		if path != "" && strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "     ") && strings.HasSuffix(line, ":") {
			m := strings.TrimSuffix(strings.TrimSpace(line), ":")
			if methods[m] {
				documented[strings.ToUpper(m)+" "+path] = true
			}
		}
	}
	e := newEnv(t)
	routes := map[string]bool{}
	for _, r := range e.srv.Routes() {
		routes[r] = true
		if !documented[r] {
			t.Errorf("route not documented in openapi.yaml: %s", r)
		}
	}
	for d := range documented {
		if !routes[d] {
			t.Errorf("openapi.yaml documents a route that does not exist: %s", d)
		}
	}
}
```

- [ ] **Step 2: Uruchom testy i potwierdź, że nie przechodzą**

Run: `go test ./internal/httpapi/ 2>&1 | head`
Expected: FAIL (`undefined: httpapi.New` / brak pakietu).

- [ ] **Step 3: `errors.go`**

```go
package httpapi

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"

	"mealplan/internal/auth"
	"mealplan/internal/store"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Code: code, Message: msg})
}

// fail maps service errors to HTTP responses. Unknown errors become 500 and are logged.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var rl *auth.RateLimitedError
	switch {
	case errors.As(err, &rl):
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(rl.RetryAfter.Seconds()))))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid login or password")
	case errors.Is(err, auth.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
	case errors.Is(err, auth.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
	case errors.Is(err, auth.ErrSetupDone):
		writeError(w, http.StatusConflict, "setup_done", "setup has already been completed")
	case errors.Is(err, auth.ErrInviteInvalid):
		writeError(w, http.StatusNotFound, "invite_invalid", "invite is invalid, used or expired")
	case errors.Is(err, auth.ErrLoginTaken):
		writeError(w, http.StatusConflict, "login_taken", "login is already taken")
	case errors.Is(err, auth.ErrPasswordLength):
		writeError(w, http.StatusUnprocessableEntity, "password_length", "password must be 12 to 128 characters")
	case errors.Is(err, auth.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	default:
		s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}
```

- [ ] **Step 4: `middleware.go`**

```go
package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"mealplan/internal/auth"
)

const (
	sessionCookie = "mealplan_session"
	csrfCookie    = "mealplan_csrf"
	csrfHeader    = "X-CSRF-Token"
)

type ctxKey int

const principalKey ctxKey = 1

func principalFrom(r *http.Request) auth.Principal {
	p, _ := r.Context().Value(principalKey).(auth.Principal)
	return p
}

// Script sources are strict. style-src allows 'unsafe-inline' because Vuetify injects its
// theme stylesheet at runtime; replace it with a nonce in a later stage (see docs/security.md).
const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", csp)
		if s.cfg.CookieSecure {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic", "path", r.URL.Path, "panic", rec)
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// public wraps handlers that need no session. They accept only JSON bodies, which blocks
// cross-site form posts (a "login CSRF") without needing a token before the user has a session.
func (s *Server) public(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
			return
		}
		h(w, r)
	})
}

// private wraps handlers that need a session; unsafe methods also need a matching CSRF token.
func (s *Server) private(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		raw := ""
		if err == nil {
			raw = c.Value
		}
		p, err := s.auth.Authenticate(r.Context(), raw)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			cc, cerr := r.Cookie(csrfCookie)
			hdr := r.Header.Get(csrfHeader)
			if cerr != nil || hdr == "" ||
				subtle.ConstantTimeCompare([]byte(hdr), []byte(cc.Value)) != 1 ||
				subtle.ConstantTimeCompare([]byte(hdr), []byte(p.CSRFToken)) != 1 {
				writeError(w, http.StatusForbidden, "csrf", "missing or invalid CSRF token")
				return
			}
		}
		h(w, r.WithContext(context.WithValue(r.Context(), principalKey, p)))
	})
}
```

- [ ] **Step 5: `server.go`**

```go
// Package httpapi exposes the REST API under /api/v1 and mounts the static frontend.
package httpapi

import (
	"log/slog"
	"net/http"

	"mealplan/internal/auth"
	"mealplan/internal/config"
)

type Server struct {
	auth   *auth.Service
	cfg    config.Config
	static http.Handler
	log    *slog.Logger
}

func New(a *auth.Service, cfg config.Config, static http.Handler, log *slog.Logger) *Server {
	return &Server{auth: a, cfg: cfg, static: static, log: log}
}

type route struct {
	pattern string
	handler http.Handler
}

func (s *Server) routes() []route {
	return []route{
		{"GET /api/v1/auth/status", http.HandlerFunc(s.handleStatus)},
		{"POST /api/v1/auth/setup", s.public(s.handleSetup)},
		{"POST /api/v1/auth/login", s.public(s.handleLogin)},
		{"POST /api/v1/auth/logout", s.private(s.handleLogout)},
		{"GET /api/v1/auth/me", s.private(s.handleMe)},
		{"POST /api/v1/auth/invites", s.private(s.handleCreateInvite)},
		{"GET /api/v1/auth/invites", s.private(s.handleListInvites)},
		{"DELETE /api/v1/auth/invites/{id}", s.private(s.handleRevokeInvite)},
		{"GET /api/v1/auth/invitations/{token}", http.HandlerFunc(s.handleInviteInfo)},
		{"POST /api/v1/auth/invitations/{token}/accept", s.public(s.handleAcceptInvite)},
		{"GET /api/v1/household/members", s.private(s.handleMembers)},
	}
}

// Routes lists the registered API route patterns (used to keep openapi.yaml in sync).
func (s *Server) Routes() []string {
	var out []string
	for _, r := range s.routes() {
		out = append(out, r.pattern)
	}
	return out
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	for _, r := range s.routes() {
		mux.Handle(r.pattern, r.handler)
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such API endpoint")
	})
	mux.Handle("/", s.static)
	return s.recoverer(s.securityHeaders(mux))
}
```

- [ ] **Step 6: `handlers.go`**

```go
package httpapi

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"mealplan/internal/auth"
	"mealplan/internal/domain"
)

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large", "request body too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) setSession(w http.ResponseWriter, g auth.SessionGrant) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: g.Token, Path: "/", Expires: g.ExpiresAt,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: g.CSRFToken, Path: "/", Expires: g.ExpiresAt,
		HttpOnly: false, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
}

func (s *Server) clearSession(w http.ResponseWriter) {
	for _, name := range []string{sessionCookie, csrfCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1,
			HttpOnly: name == sessionCookie, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	need, err := s.auth.SetupRequired(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"setupRequired": need, "mode": string(s.cfg.Mode)})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		HouseholdName string `json:"householdName"`
		Login         string `json:"login"`
		Password      string `json:"password"`
		DisplayName   string `json:"displayName"`
	}
	if !decode(w, r, &in) {
		return
	}
	g, err := s.auth.Setup(r.Context(), auth.SetupInput{
		HouseholdName: in.HouseholdName, Login: in.Login, Password: in.Password, DisplayName: in.DisplayName})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSession(w, g)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	g, err := s.auth.Login(r.Context(), in.Login, in.Password, clientIP(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSession(w, g)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.auth.Logout(r.Context(), c.Value); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	s.clearSession(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"user":      map[string]string{"id": p.UserID, "login": p.Login, "displayName": p.DisplayName},
		"household": map[string]string{"id": p.HouseholdID, "name": p.HouseholdName},
		"role":      string(p.Role),
		"csrfToken": p.CSRFToken,
	})
}

type inviteDTO struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Role string `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	role := domain.Role(in.Role)
	if in.Role == "" {
		role = domain.RoleMember
	}
	raw, inv, err := s.auth.CreateInvite(r.Context(), principalFrom(r), role)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": inv.ID, "token": raw, "role": string(inv.Role), "expiresAt": inv.ExpiresAt, "path": "/invite/" + raw,
	})
}

func (s *Server) handleListInvites(w http.ResponseWriter, r *http.Request) {
	list, err := s.auth.ListInvites(r.Context(), principalFrom(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]inviteDTO, 0, len(list))
	for _, inv := range list {
		out = append(out, inviteDTO{ID: inv.ID, Role: string(inv.Role), ExpiresAt: inv.ExpiresAt})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRevokeInvite(w http.ResponseWriter, r *http.Request) {
	if err := s.auth.RevokeInvite(r.Context(), principalFrom(r), r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleInviteInfo(w http.ResponseWriter, r *http.Request) {
	info, err := s.auth.InviteInfo(r.Context(), r.PathValue("token"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"householdName": info.HouseholdName, "role": string(info.Role), "expiresAt": info.ExpiresAt,
	})
}

func (s *Server) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Login       string `json:"login"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if !decode(w, r, &in) {
		return
	}
	g, err := s.auth.AcceptInvite(r.Context(), r.PathValue("token"),
		auth.AcceptInput{Login: in.Login, Password: in.Password, DisplayName: in.DisplayName})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSession(w, g)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMembers(w http.ResponseWriter, r *http.Request) {
	members, err := s.auth.Members(r.Context(), principalFrom(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]map[string]string, 0, len(members))
	for _, m := range members {
		out = append(out, map[string]string{
			"id": m.User.ID, "login": m.User.Login, "displayName": m.User.DisplayName, "role": string(m.Role),
		})
	}
	writeJSON(w, http.StatusOK, out)
}
```

- [ ] **Step 7: Napisz `api/openapi.yaml`**

```yaml
openapi: 3.0.3
info:
  title: Mealplan API
  version: 0.1.0
  description: REST API of Mealplan. All responses are JSON; errors use the Error schema.
servers:
  - url: /
paths:
  /api/v1/auth/status:
    get:
      summary: Setup state and deployment mode
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Status" }
  /api/v1/auth/setup:
    post:
      summary: Create the first household and admin (only when no users exist)
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/SetupRequest" }
      responses:
        "204": { description: Created; session cookies set }
        "409": { $ref: "#/components/responses/Error" }
        "422": { $ref: "#/components/responses/Error" }
  /api/v1/auth/login:
    post:
      summary: Log in with login and password
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/LoginRequest" }
      responses:
        "204": { description: Logged in; session cookies set }
        "401": { $ref: "#/components/responses/Error" }
        "429": { $ref: "#/components/responses/Error" }
  /api/v1/auth/logout:
    post:
      summary: End the current session
      security: [{ session: [], csrf: [] }]
      responses:
        "204": { description: Logged out }
  /api/v1/auth/me:
    get:
      summary: Current user, household, role and CSRF token
      security: [{ session: [] }]
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Me" }
        "401": { $ref: "#/components/responses/Error" }
  /api/v1/auth/invites:
    post:
      summary: Create a one-time invite (admin)
      security: [{ session: [], csrf: [] }]
      requestBody:
        content:
          application/json:
            schema: { $ref: "#/components/schemas/CreateInviteRequest" }
      responses:
        "201":
          description: Created; the token is shown only once
          content:
            application/json:
              schema: { $ref: "#/components/schemas/CreatedInvite" }
        "403": { $ref: "#/components/responses/Error" }
    get:
      summary: List pending invites (admin)
      security: [{ session: [] }]
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: array
                items: { $ref: "#/components/schemas/Invite" }
        "403": { $ref: "#/components/responses/Error" }
  /api/v1/auth/invites/{id}:
    delete:
      summary: Revoke a pending invite of the own household (admin)
      security: [{ session: [], csrf: [] }]
      parameters:
        - { name: id, in: path, required: true, schema: { type: string } }
      responses:
        "204": { description: Revoked }
        "404": { $ref: "#/components/responses/Error" }
  /api/v1/auth/invitations/{token}:
    get:
      summary: Inspect an invite link
      parameters:
        - { name: token, in: path, required: true, schema: { type: string } }
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: { $ref: "#/components/schemas/InviteInfo" }
        "404": { $ref: "#/components/responses/Error" }
  /api/v1/auth/invitations/{token}/accept:
    post:
      summary: Accept an invite and create an account
      parameters:
        - { name: token, in: path, required: true, schema: { type: string } }
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/AcceptRequest" }
      responses:
        "204": { description: Account created; session cookies set }
        "404": { $ref: "#/components/responses/Error" }
        "409": { $ref: "#/components/responses/Error" }
        "422": { $ref: "#/components/responses/Error" }
  /api/v1/household/members:
    get:
      summary: Members of the current household
      security: [{ session: [] }]
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: array
                items: { $ref: "#/components/schemas/Member" }
components:
  securitySchemes:
    session: { type: apiKey, in: cookie, name: mealplan_session }
    csrf: { type: apiKey, in: header, name: X-CSRF-Token }
  responses:
    Error:
      description: Error
      content:
        application/json:
          schema: { $ref: "#/components/schemas/Error" }
  schemas:
    Error:
      type: object
      required: [code, message]
      properties:
        code: { type: string }
        message: { type: string }
        details: {}
    Status:
      type: object
      properties:
        setupRequired: { type: boolean }
        mode: { type: string, enum: [selfhost, hosted] }
    SetupRequest:
      type: object
      required: [householdName, login, password, displayName]
      properties:
        householdName: { type: string, maxLength: 100 }
        login: { type: string, pattern: "^[a-z0-9._-]{3,64}$" }
        password: { type: string, minLength: 12, maxLength: 128 }
        displayName: { type: string, maxLength: 64 }
    LoginRequest:
      type: object
      required: [login, password]
      properties:
        login: { type: string }
        password: { type: string }
    AcceptRequest:
      type: object
      required: [login, password, displayName]
      properties:
        login: { type: string, pattern: "^[a-z0-9._-]{3,64}$" }
        password: { type: string, minLength: 12, maxLength: 128 }
        displayName: { type: string, maxLength: 64 }
    Me:
      type: object
      properties:
        user:
          type: object
          properties:
            id: { type: string }
            login: { type: string }
            displayName: { type: string }
        household:
          type: object
          properties:
            id: { type: string }
            name: { type: string }
        role: { type: string, enum: [admin, member] }
        csrfToken: { type: string }
    CreateInviteRequest:
      type: object
      properties:
        role: { type: string, enum: [admin, member], default: member }
    CreatedInvite:
      type: object
      properties:
        id: { type: string }
        token: { type: string }
        role: { type: string }
        expiresAt: { type: string, format: date-time }
        path: { type: string }
    Invite:
      type: object
      properties:
        id: { type: string }
        role: { type: string }
        expiresAt: { type: string, format: date-time }
    InviteInfo:
      type: object
      properties:
        householdName: { type: string }
        role: { type: string }
        expiresAt: { type: string, format: date-time }
    Member:
      type: object
      properties:
        id: { type: string }
        login: { type: string }
        displayName: { type: string }
        role: { type: string, enum: [admin, member] }
```

- [ ] **Step 8: Zaktualizuj spec**

W `docs/superpowers/specs/2026-11-01-mealplan-design.md`, sekcja 6, diagram zaproszeń: zamień `GET /auth/invites/{token}` na `GET /auth/invitations/{token}` oraz `POST /auth/invites/{token}/accept` na `POST /auth/invitations/{token}/accept` (OpenAPI nie dopuszcza dwóch ścieżek różniących się tylko nazwą parametru). W sekcji 6, punkt "Nagłówki", dopisz po "CSP bez `unsafe-inline`": ` dla skryptów (dla stylów `unsafe-inline` jest tymczasowo dozwolone, bo Vuetify wstrzykuje arkusz motywu w czasie działania; docelowo nonce)`.

- [ ] **Step 9: Uruchom wszystkie testy Go**

Run: `gofmt -w . && go vet ./... && go test ./... -race`
Expected: wszystkie pakiety `ok`, w tym `TestRoutesMatchOpenAPI`.

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "feat(httpapi): REST auth and invite API with OpenAPI contract"
```

---

### Task 7: Powłoka frontendu (Vue 3 + Vuetify)

**Files:**
- Create: `web/package.json`, `web/tsconfig.json`, `web/vite.config.ts`, `web/index.html`, `web/src/env.d.ts`
- Create: `web/src/main.ts`, `web/src/App.vue`, `web/src/router.ts`, `web/src/guard.ts`, `web/src/messages.ts`, `web/src/plugins/vuetify.ts`
- Create: `web/src/api/client.ts`, `web/src/stores/session.ts`
- Create: `web/src/layouts/AppShell.vue`
- Create: `web/src/views/LoginView.vue`, `SetupView.vue`, `InviteAcceptView.vue`, `HomeView.vue`, `MembersView.vue`
- Test: `web/src/api/client.test.ts`, `web/src/stores/session.test.ts`, `web/src/guard.test.ts`

**Interfaces:**
- Consumes: kształty JSON i ciastka z Task 6.
- Produces: `api<T>(method, path, body?)`, `ApiError{status, code, message, retryAfter?}`, `setCsrfToken(t)`; `useSession()` ze stanem `me`, `loaded`, `setupRequired` i akcjami `bootstrap`, `fetchMe`, `login`, `setup`, `acceptInvite`, `logout`; `resolveRoute(state, name, isPublic)`; `errorText(e)` i obiekt `t` w `messages.ts`. Build trafia do `web/dist`.

- [ ] **Step 1: Konfiguracja projektu**

`web/package.json`:

```json
{
  "name": "mealplan-web",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "test": "vitest run"
  },
  "dependencies": {
    "@mdi/font": "^7.4.47",
    "pinia": "^2.2.6",
    "vue": "^3.5.13",
    "vue-router": "^4.4.5",
    "vuetify": "^3.7.4"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.2.0",
    "jsdom": "^25.0.1",
    "typescript": "^5.6.3",
    "vite": "^5.4.11",
    "vite-plugin-vuetify": "^2.0.4",
    "vitest": "^2.1.5",
    "vue-tsc": "^2.1.10"
  }
}
```

`web/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "jsx": "preserve",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "skipLibCheck": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "types": ["vite/client", "vuetify"]
  },
  "include": ["src/**/*.ts", "src/**/*.vue"]
}
```

`web/vite.config.ts`:

```ts
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import vuetify from 'vite-plugin-vuetify'

export default defineConfig({
  plugins: [vue(), vuetify({ autoImport: true })],
  server: { proxy: { '/api': 'http://localhost:8080' } },
  build: { outDir: 'dist', emptyOutDir: true },
  test: { environment: 'jsdom' },
})
```

`web/index.html`:

```html
<!doctype html>
<html lang="pl">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta name="theme-color" content="#2E7D32" />
    <title>Mealplan</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

`web/src/env.d.ts`:

```ts
/// <reference types="vite/client" />
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<object, object, unknown>
  export default component
}
```

```bash
cd web && npm install
```

- [ ] **Step 2: Napisz testy (mają nie przejść)**

`web/src/api/client.test.ts`:

```ts
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError, setCsrfToken } from './client'

function reply(status: number, body?: unknown, headers: Record<string, string> = {}) {
  return Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), { status, headers }))
}

afterEach(() => {
  vi.unstubAllGlobals()
  setCsrfToken('')
})

describe('api client', () => {
  it('sends JSON bodies and the CSRF header on writes only', async () => {
    const fetchMock = vi.fn(() => reply(200, { ok: true }))
    vi.stubGlobal('fetch', fetchMock)
    setCsrfToken('tok')
    await api('POST', '/auth/invites', { role: 'member' })
    await api('GET', '/auth/me')
    const [, post] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    const [, get] = fetchMock.mock.calls[1] as unknown as [string, RequestInit]
    expect((post.headers as Record<string, string>)['X-CSRF-Token']).toBe('tok')
    expect((post.headers as Record<string, string>)['Content-Type']).toBe('application/json')
    expect(post.body).toBe('{"role":"member"}')
    expect((get.headers as Record<string, string>)['X-CSRF-Token']).toBeUndefined()
  })

  it('returns undefined for 204', async () => {
    vi.stubGlobal('fetch', vi.fn(() => reply(204)))
    expect(await api('POST', '/auth/logout')).toBeUndefined()
  })

  it('throws ApiError with code, message and Retry-After', async () => {
    vi.stubGlobal('fetch', vi.fn(() => reply(429, { code: 'rate_limited', message: 'slow down' }, { 'Retry-After': '7' })))
    await expect(api('POST', '/auth/login', {})).rejects.toMatchObject({
      status: 429, code: 'rate_limited', message: 'slow down', retryAfter: 7,
    })
  })

  it('wraps non-JSON failures', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response('<html>', { status: 502 }))))
    const err = await api('GET', '/auth/me').catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(502)
  })
})
```

`web/src/stores/session.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useSession } from './session'
import { ApiError } from '../api/client'

const me = {
  user: { id: 'u1', login: 'marc', displayName: 'Marc' },
  household: { id: 'h1', name: 'Dom' },
  role: 'admin',
  csrfToken: 'csrf',
}

function json(status: number, body?: unknown) {
  return Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), { status }))
}

function route(map: Record<string, () => Promise<Response>>) {
  return vi.fn((url: string, init?: RequestInit) => {
    const handler = map[`${init?.method ?? 'GET'} ${url}`]
    if (!handler) throw new Error(`unexpected request ${init?.method} ${url}`)
    return handler()
  })
}

beforeEach(() => setActivePinia(createPinia()))
afterEach(() => vi.unstubAllGlobals())

describe('session store', () => {
  it('bootstrap loads status and the current user', async () => {
    vi.stubGlobal('fetch', route({
      'GET /api/v1/auth/status': () => json(200, { setupRequired: false, mode: 'selfhost' }),
      'GET /api/v1/auth/me': () => json(200, me),
    }))
    const s = useSession()
    await s.bootstrap()
    expect(s.loaded).toBe(true)
    expect(s.me?.user.displayName).toBe('Marc')
    expect(s.isAdmin).toBe(true)
  })

  it('bootstrap treats 401 as logged out', async () => {
    vi.stubGlobal('fetch', route({
      'GET /api/v1/auth/status': () => json(200, { setupRequired: true, mode: 'selfhost' }),
      'GET /api/v1/auth/me': () => json(401, { code: 'unauthenticated', message: 'x' }),
    }))
    const s = useSession()
    await s.bootstrap()
    expect(s.me).toBeNull()
    expect(s.setupRequired).toBe(true)
  })

  it('login fetches the profile afterwards', async () => {
    vi.stubGlobal('fetch', route({
      'POST /api/v1/auth/login': () => json(204),
      'GET /api/v1/auth/me': () => json(200, me),
    }))
    const s = useSession()
    await s.login('marc', 'pw')
    expect(s.me?.household.name).toBe('Dom')
  })

  it('reports a dropped session cookie (Secure cookie over plain HTTP)', async () => {
    vi.stubGlobal('fetch', route({
      'POST /api/v1/auth/login': () => json(204),
      'GET /api/v1/auth/me': () => json(401, { code: 'unauthenticated', message: 'x' }),
    }))
    const s = useSession()
    const err = await s.login('marc', 'pw').catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err.code).toBe('cookie_not_stored')
  })

  it('setup clears setupRequired and logout clears the user', async () => {
    vi.stubGlobal('fetch', route({
      'POST /api/v1/auth/setup': () => json(204),
      'GET /api/v1/auth/me': () => json(200, me),
      'POST /api/v1/auth/logout': () => json(204),
    }))
    const s = useSession()
    s.setupRequired = true
    await s.setup({ householdName: 'Dom', login: 'marc', password: 'pw', displayName: 'Marc' })
    expect(s.setupRequired).toBe(false)
    expect(s.me).not.toBeNull()
    await s.logout()
    expect(s.me).toBeNull()
  })
})
```

`web/src/guard.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { resolveRoute } from './guard'

const me = { id: 'u1' }

describe('resolveRoute', () => {
  it('forces setup while no users exist', () => {
    expect(resolveRoute({ me: null, setupRequired: true }, 'login', true)).toEqual({ name: 'setup' })
    expect(resolveRoute({ me: null, setupRequired: true }, 'setup', true)).toBe(true)
  })
  it('hides setup once it is done', () => {
    expect(resolveRoute({ me: null, setupRequired: false }, 'setup', true)).toEqual({ name: 'login' })
    expect(resolveRoute({ me, setupRequired: false }, 'setup', true)).toEqual({ name: 'home' })
  })
  it('sends anonymous users to login but lets public pages through', () => {
    expect(resolveRoute({ me: null, setupRequired: false }, 'home', false)).toEqual({ name: 'login' })
    expect(resolveRoute({ me: null, setupRequired: false }, 'invite', true)).toBe(true)
  })
  it('keeps signed-in users out of the login page', () => {
    expect(resolveRoute({ me, setupRequired: false }, 'login', true)).toEqual({ name: 'home' })
    expect(resolveRoute({ me, setupRequired: false }, 'members', false)).toBe(true)
  })
})
```

- [ ] **Step 3: Uruchom testy i potwierdź, że nie przechodzą**

Run: `cd web && npx vitest run`
Expected: FAIL (brak modułów `./client`, `./session`, `./guard`).

- [ ] **Step 4: Klient API**

`web/src/api/client.ts`:

```ts
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public retryAfter?: number,
  ) {
    super(message)
  }
}

let csrfToken = ''

export function setCsrfToken(token: string) {
  csrfToken = token
}

export async function api<T = void>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (method !== 'GET' && csrfToken) headers['X-CSRF-Token'] = csrfToken
  const res = await fetch('/api/v1' + path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: 'same-origin',
  })
  if (res.status === 204) return undefined as T
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new ApiError(
      res.status,
      data.code ?? 'unknown',
      data.message ?? res.statusText,
      Number(res.headers.get('Retry-After')) || undefined,
    )
  }
  return data as T
}
```

- [ ] **Step 5: Store sesji i guard**

`web/src/stores/session.ts`:

```ts
import { defineStore } from 'pinia'
import { api, ApiError, setCsrfToken } from '../api/client'

export interface Me {
  user: { id: string; login: string; displayName: string }
  household: { id: string; name: string }
  role: 'admin' | 'member'
  csrfToken: string
}

interface Status {
  setupRequired: boolean
  mode: 'selfhost' | 'hosted'
}

export const useSession = defineStore('session', {
  state: () => ({ me: null as Me | null, loaded: false, setupRequired: false }),
  getters: {
    isAdmin: (s) => s.me?.role === 'admin',
  },
  actions: {
    async bootstrap() {
      const status = await api<Status>('GET', '/auth/status')
      this.setupRequired = status.setupRequired
      try {
        await this.fetchMe()
      } catch (e) {
        if (!(e instanceof ApiError && e.status === 401)) throw e
        this.me = null
      }
      this.loaded = true
    },
    async fetchMe() {
      const me = await api<Me>('GET', '/auth/me')
      this.me = me
      setCsrfToken(me.csrfToken)
    },
    // After a successful sign-in the profile must load; a 401 here means the browser dropped
    // the cookie, typically a Secure cookie served over plain HTTP.
    async afterSignIn() {
      try {
        await this.fetchMe()
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) {
          throw new ApiError(0, 'cookie_not_stored', 'Session cookie was not stored by the browser')
        }
        throw e
      }
    },
    async login(login: string, password: string) {
      await api('POST', '/auth/login', { login, password })
      await this.afterSignIn()
    },
    async setup(input: { householdName: string; login: string; password: string; displayName: string }) {
      await api('POST', '/auth/setup', input)
      await this.afterSignIn()
      this.setupRequired = false
    },
    async acceptInvite(token: string, input: { login: string; password: string; displayName: string }) {
      await api('POST', `/auth/invitations/${encodeURIComponent(token)}/accept`, input)
      await this.afterSignIn()
    },
    async logout() {
      await api('POST', '/auth/logout')
      this.me = null
      setCsrfToken('')
    },
  },
})
```

`web/src/guard.ts`:

```ts
export interface GuardState {
  me: unknown | null
  setupRequired: boolean
}

export function resolveRoute(state: GuardState, name: string | undefined, isPublic: boolean): true | { name: string } {
  if (state.setupRequired) return name === 'setup' ? true : { name: 'setup' }
  if (name === 'setup') return state.me ? { name: 'home' } : { name: 'login' }
  if (!state.me) return isPublic ? true : { name: 'login' }
  if (name === 'login') return { name: 'home' }
  return true
}
```

- [ ] **Step 6: Uruchom testy**

Run: `cd web && npx vitest run`
Expected: PASS (13 testów w 3 plikach).

- [ ] **Step 7: Teksty i motyw**

`web/src/messages.ts`:

```ts
import { ApiError } from './api/client'

export const t = {
  appName: 'Mealplan',
  login: 'Login',
  password: 'Hasło',
  passwordHint: 'Co najmniej 12 znaków',
  displayName: 'Imię',
  householdName: 'Nazwa gospodarstwa',
  signIn: 'Zaloguj',
  signOut: 'Wyloguj',
  setupTitle: 'Pierwsze uruchomienie',
  setupIntro: 'Utwórz gospodarstwo i konto administratora.',
  createAccount: 'Utwórz konto',
  inviteTitle: 'Dołącz do gospodarstwa',
  inviteInvalid: 'To zaproszenie jest nieprawidłowe, zostało użyte lub wygasło.',
  navPlan: 'Plan',
  navMembers: 'Domownicy',
  homeSoon: 'Plan tygodnia pojawi się w kolejnym etapie.',
  members: 'Domownicy',
  admin: 'Administrator',
  member: 'Domownik',
  invites: 'Zaproszenia',
  newInvite: 'Nowe zaproszenie',
  role: 'Rola',
  createInvite: 'Utwórz link',
  copy: 'Kopiuj',
  copied: 'Skopiowano',
  inviteLinkOnce: 'Link jest pokazywany tylko raz. Skopiuj go teraz.',
  noInvites: 'Brak oczekujących zaproszeń.',
  expires: 'Ważne do',
  revoke: 'Unieważnij',
  theme: 'Zmień motyw',
}

const errors: Record<string, string> = {
  invalid_credentials: 'Nieprawidłowy login lub hasło.',
  rate_limited: 'Zbyt wiele prób. Spróbuj ponownie za chwilę.',
  setup_done: 'Konfiguracja została już wykonana.',
  invite_invalid: t.inviteInvalid,
  login_taken: 'Ten login jest już zajęty.',
  password_length: 'Hasło musi mieć od 12 do 128 znaków.',
  invalid_input: 'Sprawdź poprawność wpisanych danych.',
  forbidden: 'Brak uprawnień do tej operacji.',
  cookie_not_stored:
    'Przeglądarka nie zapisała sesji. Jeśli otwierasz aplikację przez zwykłe HTTP, ustaw MEALPLAN_COOKIE_SECURE=false lub użyj HTTPS (np. tailscale serve).',
}

export function errorText(e: unknown): string {
  if (e instanceof ApiError) return errors[e.code] ?? e.message
  return 'Coś poszło nie tak. Spróbuj ponownie.'
}
```

`web/src/plugins/vuetify.ts`:

```ts
import { createVuetify } from 'vuetify'

export const vuetify = createVuetify({
  theme: {
    defaultTheme: 'system',
    themes: {
      light: { colors: { primary: '#2E7D32', secondary: '#546E7A' } },
      dark: { colors: { primary: '#81C784', secondary: '#90A4AE' } },
    },
  },
  defaults: {
    VBtn: { variant: 'flat' },
    VTextField: { variant: 'outlined', density: 'comfortable' },
    VSelect: { variant: 'outlined', density: 'comfortable' },
  },
})
```

- [ ] **Step 8: Router, wejście aplikacji, widoki**

`web/src/router.ts`:

```ts
import { createRouter, createWebHistory } from 'vue-router'
import { resolveRoute } from './guard'
import { useSession } from './stores/session'
import AppShell from './layouts/AppShell.vue'
import HomeView from './views/HomeView.vue'
import InviteAcceptView from './views/InviteAcceptView.vue'
import LoginView from './views/LoginView.vue'
import MembersView from './views/MembersView.vue'
import SetupView from './views/SetupView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/setup', name: 'setup', component: SetupView, meta: { public: true } },
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    { path: '/invite/:token', name: 'invite', component: InviteAcceptView, meta: { public: true } },
    {
      path: '/',
      component: AppShell,
      children: [
        { path: '', name: 'home', component: HomeView },
        { path: 'members', name: 'members', component: MembersView },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  const session = useSession()
  if (!session.loaded) await session.bootstrap()
  return resolveRoute(
    { me: session.me, setupRequired: session.setupRequired },
    to.name as string | undefined,
    Boolean(to.meta.public),
  )
})
```

`web/src/main.ts`:

```ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import App from './App.vue'
import { router } from './router'
import { vuetify } from './plugins/vuetify'

createApp(App).use(createPinia()).use(router).use(vuetify).mount('#app')
```

`web/src/App.vue`:

```vue
<template>
  <v-app>
    <router-view />
  </v-app>
</template>
```

`web/src/layouts/AppShell.vue`:

```vue
<script setup lang="ts">
import { useDisplay, useTheme } from 'vuetify'
import { useRouter } from 'vue-router'
import { useSession } from '../stores/session'
import { t } from '../messages'

const { mdAndUp } = useDisplay()
const theme = useTheme()
const session = useSession()
const router = useRouter()

const items = [
  { title: t.navPlan, icon: 'mdi-calendar-week', to: { name: 'home' } },
  { title: t.navMembers, icon: 'mdi-account-group', to: { name: 'members' } },
]

function toggleTheme() {
  theme.global.name.value = theme.global.current.value.dark ? 'light' : 'dark'
}

async function logout() {
  await session.logout()
  await router.push({ name: 'login' })
}
</script>

<template>
  <v-app-bar flat border="b">
    <v-app-bar-title>{{ t.appName }}</v-app-bar-title>
    <span class="text-body-2 mr-2 d-none d-sm-inline">{{ session.me?.household.name }}</span>
    <v-btn icon variant="text" :title="t.theme" @click="toggleTheme"><v-icon>mdi-theme-light-dark</v-icon></v-btn>
    <v-btn icon variant="text" :title="t.signOut" @click="logout"><v-icon>mdi-logout</v-icon></v-btn>
  </v-app-bar>

  <v-navigation-drawer v-if="mdAndUp" rail permanent>
    <v-list nav>
      <v-list-item v-for="i in items" :key="i.title" :to="i.to" :prepend-icon="i.icon" :title="i.title" />
    </v-list>
  </v-navigation-drawer>

  <v-main>
    <v-container class="py-6" style="max-width: 960px"><router-view /></v-container>
  </v-main>

  <v-bottom-navigation v-if="!mdAndUp" grow>
    <v-btn v-for="i in items" :key="i.title" :to="i.to"><v-icon>{{ i.icon }}</v-icon>{{ i.title }}</v-btn>
  </v-bottom-navigation>
</template>
```

`web/src/views/LoginView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSession } from '../stores/session'
import { errorText, t } from '../messages'

const session = useSession()
const router = useRouter()
const login = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  busy.value = true
  error.value = ''
  try {
    await session.login(login.value, password.value)
    await router.push({ name: 'home' })
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <v-main class="d-flex align-center justify-center">
    <v-card width="400" max-width="92vw" class="pa-4" elevation="2">
      <v-card-title>{{ t.appName }}</v-card-title>
      <v-form @submit.prevent="submit">
        <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
        <v-text-field v-model="login" :label="t.login" autocomplete="username" autofocus />
        <v-text-field v-model="password" :label="t.password" type="password" autocomplete="current-password" />
        <v-btn type="submit" color="primary" block :loading="busy">{{ t.signIn }}</v-btn>
      </v-form>
    </v-card>
  </v-main>
</template>
```

`web/src/views/SetupView.vue`:

```vue
<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSession } from '../stores/session'
import { errorText, t } from '../messages'

const session = useSession()
const router = useRouter()
const form = reactive({ householdName: '', displayName: '', login: '', password: '' })
const error = ref('')
const busy = ref(false)

async function submit() {
  busy.value = true
  error.value = ''
  try {
    await session.setup({ ...form })
    await router.push({ name: 'home' })
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <v-main class="d-flex align-center justify-center">
    <v-card width="440" max-width="92vw" class="pa-4" elevation="2">
      <v-card-title>{{ t.setupTitle }}</v-card-title>
      <v-card-subtitle class="mb-4">{{ t.setupIntro }}</v-card-subtitle>
      <v-form @submit.prevent="submit">
        <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
        <v-text-field v-model="form.householdName" :label="t.householdName" autofocus />
        <v-text-field v-model="form.displayName" :label="t.displayName" />
        <v-text-field v-model="form.login" :label="t.login" autocomplete="username" />
        <v-text-field v-model="form.password" :label="t.password" :hint="t.passwordHint" persistent-hint type="password" autocomplete="new-password" class="mb-4" />
        <v-btn type="submit" color="primary" block :loading="busy">{{ t.createAccount }}</v-btn>
      </v-form>
    </v-card>
  </v-main>
</template>
```

`web/src/views/InviteAcceptView.vue`:

```vue
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { useSession } from '../stores/session'
import { errorText, t } from '../messages'

const route = useRoute()
const router = useRouter()
const session = useSession()
const token = String(route.params.token)
const householdName = ref('')
const valid = ref<boolean | null>(null)
const form = reactive({ displayName: '', login: '', password: '' })
const error = ref('')
const busy = ref(false)

onMounted(async () => {
  try {
    const info = await api<{ householdName: string }>('GET', `/auth/invitations/${encodeURIComponent(token)}`)
    householdName.value = info.householdName
    valid.value = true
  } catch {
    valid.value = false
  }
})

async function submit() {
  busy.value = true
  error.value = ''
  try {
    await session.acceptInvite(token, { ...form })
    await router.push({ name: 'home' })
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <v-main class="d-flex align-center justify-center">
    <v-card width="440" max-width="92vw" class="pa-4" elevation="2">
      <v-card-title>{{ t.inviteTitle }}</v-card-title>
      <v-alert v-if="valid === false" type="error" variant="tonal" class="mt-2">{{ t.inviteInvalid }}</v-alert>
      <v-form v-else-if="valid" @submit.prevent="submit">
        <v-card-subtitle class="mb-4">{{ householdName }}</v-card-subtitle>
        <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
        <v-text-field v-model="form.displayName" :label="t.displayName" autofocus />
        <v-text-field v-model="form.login" :label="t.login" autocomplete="username" />
        <v-text-field v-model="form.password" :label="t.password" :hint="t.passwordHint" persistent-hint type="password" autocomplete="new-password" class="mb-4" />
        <v-btn type="submit" color="primary" block :loading="busy">{{ t.createAccount }}</v-btn>
      </v-form>
    </v-card>
  </v-main>
</template>
```

`web/src/views/HomeView.vue`:

```vue
<script setup lang="ts">
import { useSession } from '../stores/session'
import { t } from '../messages'

const session = useSession()
</script>

<template>
  <h1 class="text-h5 mb-2">{{ session.me?.household.name }}</h1>
  <p class="text-medium-emphasis">{{ t.homeSoon }}</p>
</template>
```

`web/src/views/MembersView.vue`:

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
import { useSession } from '../stores/session'
import { errorText, t } from '../messages'

interface Member { id: string; login: string; displayName: string; role: 'admin' | 'member' }
interface Invite { id: string; role: 'admin' | 'member'; expiresAt: string }

const session = useSession()
const members = ref<Member[]>([])
const invites = ref<Invite[]>([])
const newRole = ref<'admin' | 'member'>('member')
const link = ref('')
const copied = ref(false)
const error = ref('')

const roles = [
  { title: t.member, value: 'member' },
  { title: t.admin, value: 'admin' },
]

async function load() {
  members.value = await api<Member[]>('GET', '/household/members')
  if (session.isAdmin) invites.value = await api<Invite[]>('GET', '/auth/invites')
}

async function create() {
  error.value = ''
  copied.value = false
  try {
    const res = await api<{ path: string }>('POST', '/auth/invites', { role: newRole.value })
    link.value = location.origin + res.path
    await load()
  } catch (e) {
    error.value = errorText(e)
  }
}

async function revoke(id: string) {
  await api('DELETE', `/auth/invites/${id}`)
  await load()
}

async function copy() {
  await navigator.clipboard.writeText(link.value)
  copied.value = true
}

onMounted(load)
</script>

<template>
  <h1 class="text-h5 mb-4">{{ t.members }}</h1>
  <v-list lines="one" border rounded class="mb-8">
    <v-list-item v-for="m in members" :key="m.id" :title="m.displayName" :subtitle="m.login">
      <template #append><v-chip size="small">{{ m.role === 'admin' ? t.admin : t.member }}</v-chip></template>
    </v-list-item>
  </v-list>

  <template v-if="session.isAdmin">
    <h2 class="text-h6 mb-2">{{ t.invites }}</h2>
    <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
    <div class="d-flex flex-wrap ga-3 align-start mb-4">
      <v-select v-model="newRole" :items="roles" :label="t.role" style="max-width: 220px" hide-details />
      <v-btn color="primary" class="mt-1" @click="create">{{ t.createInvite }}</v-btn>
    </div>
    <v-alert v-if="link" type="info" variant="tonal" class="mb-4">
      <div class="mb-2">{{ t.inviteLinkOnce }}</div>
      <code style="word-break: break-all">{{ link }}</code>
      <div class="mt-2"><v-btn size="small" @click="copy">{{ copied ? t.copied : t.copy }}</v-btn></div>
    </v-alert>
    <p v-if="!invites.length" class="text-medium-emphasis">{{ t.noInvites }}</p>
    <v-list v-else lines="one" border rounded>
      <v-list-item v-for="i in invites" :key="i.id" :title="i.role === 'admin' ? t.admin : t.member" :subtitle="`${t.expires} ${new Date(i.expiresAt).toLocaleString('pl-PL')}`">
        <template #append><v-btn size="small" variant="text" color="error" @click="revoke(i.id)">{{ t.revoke }}</v-btn></template>
      </v-list-item>
    </v-list>
  </template>
</template>
```

- [ ] **Step 9: Zbuduj frontend**

Run: `cd web && npm test && npm run build`
Expected: testy PASS; `vue-tsc` bez błędów; w `web/dist` pojawiają się `index.html` i `assets/`.

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "feat(web): Vue + Vuetify shell with auth, invites and members views"
```

---

### Task 8: Osadzenie frontendu, punkt wejścia i Docker

**Files:**
- Create: `web/embed.go`, `web/dist/.gitkeep`, `internal/spa/spa.go`, `Dockerfile`, `.dockerignore`
- Modify: `cmd/mealplan/main.go`
- Test: `internal/spa/spa_test.go`

**Interfaces:**
- Consumes: `httpapi.New`, `auth.NewService`, `store.Open`, `config.Load`.
- Produces: `web.Dist` (`embed.FS` z katalogiem `dist`), `spa.Handler(fsys fs.FS) http.Handler` (pliki statyczne, fallback do `index.html` dla ścieżek innych niż `/api/`).

- [ ] **Step 1: Napisz test handlera SPA (ma nie przejść)**

`internal/spa/spa_test.go`:

```go
package spa

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

var site = fstest.MapFS{
	"index.html":     {Data: []byte("<html>app</html>")},
	"assets/app.js":  {Data: []byte("console.log(1)")},
	"assets/app.css": {Data: []byte("body{}")},
}

func get(path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	Handler(site).ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestServesIndexAtRoot(t *testing.T) {
	rec := get("/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("index must not be cached: %q", rec.Header().Get("Cache-Control"))
	}
}

func TestServesAssetsWithLongCache(t *testing.T) {
	rec := get("/assets/app.js")
	if rec.Code != 200 || rec.Body.String() != "console.log(1)" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("assets must be immutable: %q", rec.Header().Get("Cache-Control"))
	}
}

func TestDeepLinksFallBackToIndex(t *testing.T) {
	for _, p := range []string{"/invite/abc123", "/members", "/login"} {
		rec := get(p)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") {
			t.Errorf("%s: %d %s", p, rec.Code, rec.Body)
		}
	}
}

func TestMissingAssetIsNotFound(t *testing.T) {
	if rec := get("/assets/missing.js"); rec.Code != 404 {
		t.Fatalf("missing asset should be 404, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Uruchom test i potwierdź, że nie przechodzi**

Run: `go test ./internal/spa/ 2>&1 | head`
Expected: FAIL (`undefined: Handler`).

- [ ] **Step 3: Zaimplementuj handler**

`internal/spa/spa.go`:

```go
// Package spa serves the embedded single-page app with a history-API fallback.
package spa

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func Handler(fsys fs.FS) http.Handler {
	files := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if st, err := fs.Stat(fsys, p); err == nil && !st.IsDir() {
			if strings.HasPrefix(p, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else if p == "index.html" {
				w.Header().Set("Cache-Control", "no-cache")
			}
			if p == "index.html" {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/" // FileServer redirects /index.html to ./
				files.ServeHTTP(w, r2)
				return
			}
			files.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(p, "assets/") || path.Ext(p) != "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		files.ServeHTTP(w, r2)
	})
}
```

- [ ] **Step 4: Osadzenie i punkt wejścia**

`web/embed.go`:

```go
// Package web embeds the built frontend (web/dist).
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
```

```bash
mkdir -p web/dist && touch web/dist/.gitkeep
```

`cmd/mealplan/main.go`:

```go
package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mealplan/internal/auth"
	"mealplan/internal/config"
	"mealplan/internal/httpapi"
	"mealplan/internal/spa"
	"mealplan/internal/store"
	"mealplan/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	db, err := store.Open(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := db.Migrate(ctx); err != nil {
		return err
	}
	svc, err := auth.NewService(store.New(db), auth.Config{
		SessionIdle: cfg.SessionIdle,
		SessionMax:  cfg.SessionMax,
		InviteTTL:   72 * time.Hour,
		Hasher:      auth.DefaultHasher,
	}, nil)
	if err != nil {
		return err
	}
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(svc, cfg, spa.Handler(dist), log).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "addr", cfg.Addr, "mode", cfg.Mode, "db", cfg.DBDriver)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
```

- [ ] **Step 5: Dockerfile**

`Dockerfile`:

```dockerfile
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mealplan ./cmd/mealplan

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/mealplan /mealplan
ENV MEALPLAN_ADDR=:8080 \
    MEALPLAN_DB_DSN="file:/data/mealplan.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/mealplan"]
```

`.dockerignore`:

```
.git
bin
web/node_modules
web/dist
*.db
```

- [ ] **Step 6: Uruchom testy Go**

Run: `go vet ./... && go test ./... -race`
Expected: wszystkie pakiety `ok`, w tym `internal/spa`.

- [ ] **Step 7: Weryfikacja ręczna na działającym pliku binarnym**

```bash
make build
MEALPLAN_COOKIE_SECURE=false MEALPLAN_DB_DSN="file:/tmp/mealplan-check.db?_pragma=foreign_keys(1)" ./bin/mealplan &
sleep 1
curl -s localhost:8080/api/v1/auth/status
curl -s -o /dev/null -w "%{http_code} %{content_type}\n" localhost:8080/
curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/invite/abc
curl -s -c /tmp/mp.jar -H 'Content-Type: application/json' -d '{"householdName":"Dom","login":"marc","password":"correct-horse-battery","displayName":"Marc"}' -o /dev/null -w "%{http_code}\n" localhost:8080/api/v1/auth/setup
curl -s -b /tmp/mp.jar localhost:8080/api/v1/auth/me
kill %1; rm -f /tmp/mealplan-check.db* /tmp/mp.jar
```

Expected: `{"setupRequired":true,"mode":"selfhost"}`; `200 text/html...`; `200` dla deep linku; `204` dla setup; JSON z `"role":"admin"` dla `/auth/me`.

- [ ] **Step 8: Weryfikacja w przeglądarce**

Uruchom `make run`, otwórz `http://localhost:8080`, przejdź: ekran Setup → utworzenie konta → Home → Domownicy → utworzenie zaproszenia → otwarcie linku w oknie prywatnym → rejestracja domownika → wylogowanie i logowanie. Sprawdź układ na szerokości telefonu (dolna nawigacja) i komputera (rail). Zapisz wynik w opisie commita.

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat: embed frontend, wire server entrypoint and add Dockerfile"
```

---

### Task 9: CI i dokumentacja

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `README.md` (zastępuje wersję tymczasową), `docs/architecture.md`, `docs/security.md`, `docs/modes.md`, `docs/deployment-selfhost.md`, `docs/api.md`, `CONTRIBUTING.md`, `CHANGELOG.md`
- Test: `docs/docs_test.go`

**Interfaces:**
- Consumes: wszystkie poprzednie zadania; diagramy opisują zbudowane komponenty.
- Produces: pipeline CI (macierz SQLite i Postgres), test poprawności bloków mermaid w dokumentacji.

- [ ] **Step 1: Test dokumentacji (ma nie przejść)**

`docs/docs_test.go`:

```go
package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var diagramTypes = []string{"flowchart", "sequenceDiagram", "erDiagram", "stateDiagram", "classDiagram", "graph"}

// Every ```mermaid fence in the documentation must be closed and start with a known diagram type.
func TestMermaidBlocksAreWellFormed(t *testing.T) {
	files, _ := filepath.Glob("*.md")
	files = append(files, "../README.md", "../CONTRIBUTING.md")
	blocks := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		lines := strings.Split(string(raw), "\n")
		for i := 0; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) != "```mermaid" {
				continue
			}
			blocks++
			closed, first := false, ""
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "```" {
					closed, i = true, j
					break
				}
				if first == "" && strings.TrimSpace(lines[j]) != "" {
					first = strings.TrimSpace(lines[j])
				}
			}
			if !closed {
				t.Errorf("%s: unclosed mermaid block", f)
				continue
			}
			ok := false
			for _, d := range diagramTypes {
				if strings.HasPrefix(first, d) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s: unknown mermaid diagram type %q", f, first)
			}
		}
	}
	if blocks < 4 {
		t.Fatalf("expected at least 4 mermaid diagrams in the docs, found %d", blocks)
	}
}
```

Run: `go test ./docs/ -v`
Expected: FAIL (brak plików dokumentacji / za mało diagramów).

- [ ] **Step 2: `docs/architecture.md`**

````markdown
# Architektura

Mealplan to jeden plik binarny Go z osadzonym frontendem Vue. Dane trzyma w SQLite (tryb `selfhost`) lub Postgres (tryb `hosted`, Etap 5).

## Komponenty (stan po Etapie 1)

```mermaid
flowchart LR
    subgraph Klient
        SPA[Vue 3 SPA + Vuetify]
    end
    subgraph Serwer["Plik binarny Go"]
        SPAH[spa: osadzony frontend]
        API[httpapi: /api/v1]
        AUTH[auth: hasła, sesje, zaproszenia, limiter]
        STORE[store: repozytoria i migracje]
    end
    DB[(SQLite lub Postgres)]
    SPA --> SPAH
    SPA --> API
    API --> AUTH --> STORE --> DB
```

## Model danych

```mermaid
erDiagram
    HOUSEHOLD ||--o{ MEMBERSHIP : ma
    APP_USER ||--o{ MEMBERSHIP : nalezy
    HOUSEHOLD ||--o{ INVITE : wystawia
    APP_USER ||--o{ SESSION : posiada
    HOUSEHOLD ||--o{ SESSION : aktywne
    HOUSEHOLD ||--o{ AUDIT_EVENT : loguje
    MEMBERSHIP {
        text role "admin lub member"
    }
```

Tabela użytkowników nazywa się `app_user`, bo `user` jest słowem zastrzeżonym w Postgres. Czasy są zapisane jako unix seconds (`BIGINT`). Każda przyszła tabela domenowa dostanie `household_id`.

## Pierwsze uruchomienie i zaproszenia

```mermaid
sequenceDiagram
    actor A as Pierwszy użytkownik
    participant W as SPA
    participant S as API
    A->>W: otwiera aplikację
    W->>S: GET /auth/status
    S-->>W: setupRequired = true
    A->>W: formularz konfiguracji
    W->>S: POST /auth/setup
    S-->>W: sesja (admin)
    A->>W: tworzy zaproszenie
    W->>S: POST /auth/invites
    S-->>W: link /invite/{token}
```

## Cykl życia sesji

```mermaid
stateDiagram-v2
    [*] --> Aktywna: login / setup / przyjęcie zaproszenia
    Aktywna --> Aktywna: żądanie (odświeża last_seen)
    Aktywna --> Wygasła: bezczynność > MEALPLAN_SESSION_IDLE
    Aktywna --> Wygasła: wiek > MEALPLAN_SESSION_MAX
    Aktywna --> Zakończona: wylogowanie
    Wygasła --> [*]
    Zakończona --> [*]
```

## Przepływ żądania

Żądanie przechodzi przez nagłówki bezpieczeństwa, router, a dla tras prywatnych przez uwierzytelnienie sesji i (dla metod modyfikujących) sprawdzenie CSRF. Pakiety domenowe nie znają HTTP, a `httpapi` nie zna SQL.
````

- [ ] **Step 3: `docs/security.md`**

````markdown
# Bezpieczeństwo

## Zaimplementowane w Etapie 1

| Obszar | Rozwiązanie |
|---|---|
| Hasła | argon2id (m=64 MiB, t=3, p=2), 12 do 128 znaków, ograniczenie długości przed hashowaniem |
| Sesje | token 256 bit, w bazie tylko hash SHA-256; wygasanie po bezczynności i limit bezwzględny |
| Ciastka | `HttpOnly`, `Secure` (konfigurowalne), `SameSite=Lax` |
| CSRF | double-submit: ciastko `mealplan_csrf` + nagłówek `X-CSRF-Token` zgodne z tokenem sesji |
| Trasy publiczne | tylko `Content-Type: application/json`, co blokuje formularze z innych witryn |
| Logowanie | limit na login (próg 5) i na IP (próg 30), blokada 1 s podwajana do 15 min; identyczne błędy dla złego loginu i hasła; wyrównywanie czasu odpowiedzi |
| Zaproszenia | jednorazowe, 72 godziny, token hashowany, zużycie atomowe w transakcji |
| Izolacja gospodarstw | `household_id` w zapytaniach, testy między dwoma gospodarstwami |
| Nagłówki | CSP, `X-Content-Type-Options`, `Referrer-Policy`, `X-Frame-Options`, HSTS przy `Secure` |
| Audyt | tabela `audit_event` (logowania, zaproszenia, konfiguracja) |

## Znane ograniczenia

- **CSP dla stylów** dopuszcza `unsafe-inline`, bo Vuetify wstrzykuje arkusz motywu w czasie działania. Skrypty mają ścisłe `script-src 'self'`. Docelowo: nonce.
- **Limiter prób jest w pamięci procesu.** Przy jednej instancji (`selfhost`) wystarcza. W trybie `hosted` z wieloma instancjami trzeba go przenieść do bazy (Etap 5).
- **Adres IP klienta** to `RemoteAddr`. Za reverse proxy (np. `tailscale serve`) wszystkie żądania mają adres 127.0.0.1, dlatego limit IP jest hojny, a główną ochroną jest limit na login. Zaufane proxy i `X-Forwarded-For` dojdą w Etapie 5.
- Blokada loginu pozwala atakującemu chwilowo zablokować znane konto. Blokada jest ograniczona do 15 minut.

## Zgłaszanie podatności

Opisz problem w prywatnym zgłoszeniu (GitHub Security Advisories) zamiast publicznego issue.
````

- [ ] **Step 4: `docs/modes.md`, `docs/deployment-selfhost.md`, `docs/api.md`**

`docs/modes.md`:

````markdown
# Tryby wdrożenia i konfiguracja

Jedna baza kodu, dwa tryby wybierane zmienną `MEALPLAN_MODE`.

| Obszar | `selfhost` | `hosted` (Etap 5) |
|---|---|---|
| Baza | SQLite | Postgres |
| Rejestracja | zamknięta, zaproszenia | samodzielna |
| Sieć | Tailscale | reverse proxy i własna domena |

```mermaid
flowchart TD
    S[Start aplikacji] --> C{MEALPLAN_DB_DRIVER}
    C -- sqlite --> L[Plik bazy, migracje]
    C -- postgres --> P[Połączenie pgx, migracje]
    L --> R[Serwer HTTP]
    P --> R
```

## Zmienne środowiskowe

| Zmienna | Domyślnie | Opis |
|---|---|---|
| `MEALPLAN_MODE` | `selfhost` | `selfhost` lub `hosted` |
| `MEALPLAN_ADDR` | `:8080` | adres nasłuchu |
| `MEALPLAN_DB_DRIVER` | `sqlite` | `sqlite` lub `postgres` |
| `MEALPLAN_DB_DSN` | `file:mealplan.db?...` | połączenie z bazą |
| `MEALPLAN_COOKIE_SECURE` | `true` | ciastka tylko przez HTTPS; ustaw `false` tylko do lokalnych testów po HTTP |
| `MEALPLAN_SESSION_IDLE` | `168h` | wygaśnięcie po bezczynności |
| `MEALPLAN_SESSION_MAX` | `720h` | maksymalny wiek sesji |
````

`docs/deployment-selfhost.md`:

````markdown
# Wdrożenie selfhost z Tailscale

```mermaid
flowchart LR
    T[Telefon lub komputer w tailnecie] -- HTTPS --> TS[tailscale serve]
    TS -- HTTP 127.0.0.1:8080 --> M[Mealplan]
    M --> V[(Wolumen /data z SQLite)]
```

## Docker

```bash
docker build -t mealplan .
docker run -d --name mealplan --restart unless-stopped \
  -p 127.0.0.1:8080:8080 -v mealplan-data:/data mealplan
```

## HTTPS przez Tailscale

```bash
tailscale serve --bg 8080
```

Aplikacja będzie dostępna pod adresem maszyny w tailnecie z certyfikatem HTTPS. Zostaw `MEALPLAN_COOKIE_SECURE=true`.

## Pierwsze uruchomienie

Otwórz adres aplikacji. Pierwsza osoba tworzy gospodarstwo i konto administratora, a kolejne dołączają przez linki zaproszeń (Domownicy, Zaproszenia).

## Kopia zapasowa

Zatrzymaj kontener i skopiuj plik `mealplan.db` (oraz `-wal` i `-shm`, jeśli istnieją) z wolumenu `/data`.
````

`docs/api.md`:

````markdown
# API

Kontrakt jest w [`api/openapi.yaml`](../api/openapi.yaml) i jest sprawdzany testem `TestRoutesMatchOpenAPI`: każda zarejestrowana trasa musi być opisana i odwrotnie.

Zasady ogólne:
- JSON pod `/api/v1`, błędy w formacie `{"code","message","details"}`.
- Sesja w ciastku `mealplan_session`. Żądania modyfikujące wymagają nagłówka `X-CSRF-Token` (wartość z `GET /auth/me` lub ciastka `mealplan_csrf`).

```mermaid
sequenceDiagram
    participant C as Klient
    participant S as API
    C->>S: POST /auth/login
    S-->>C: 204 + ciastka sesji i CSRF
    C->>S: GET /auth/me
    S-->>C: profil + csrfToken
    C->>S: POST /auth/invites (X-CSRF-Token)
    S-->>C: 201 + token zaproszenia
```
````

- [ ] **Step 5: `README.md`, `CONTRIBUTING.md`, `CHANGELOG.md`**

`README.md`:

````markdown
# Mealplan

Lekka domowa aplikacja do planowania jadłospisu na tydzień: kto gotuje które danie, lista zakupów, import przepisów, inwentarz i statystyki. Działa w przeglądarce na telefonie i komputerze. Licencja MIT.

> Status: Etap 1 (fundament): konta, logowanie, zaproszenia i powłoka UI. Planowanie, przepisy i zakupy w kolejnych etapach.

## Szybki start

```bash
docker build -t mealplan .
docker run -d -p 127.0.0.1:8080:8080 -v mealplan-data:/data -e MEALPLAN_COOKIE_SECURE=false mealplan
```

Otwórz `http://localhost:8080` i utwórz konto administratora. Do codziennego użytku ustaw HTTPS (np. `tailscale serve`) i zostaw `MEALPLAN_COOKIE_SECURE=true`.

Z kodu źródłowego (Go 1.23, Node 22):

```bash
make run
```

## Dokumentacja

- [Architektura](docs/architecture.md)
- [Bezpieczeństwo](docs/security.md)
- [Tryby i konfiguracja](docs/modes.md)
- [Wdrożenie selfhost z Tailscale](docs/deployment-selfhost.md)
- [API](docs/api.md)

## Licencja

MIT, zobacz [LICENSE](LICENSE).
````

`CONTRIBUTING.md`:

````markdown
# Współpraca

1. Backend: `go test ./... -race`. Frontend: `cd web && npm test`.
2. Testy bazy uruchamiają się domyślnie na SQLite. Aby przetestować Postgres: `MEALPLAN_TEST_DRIVER=postgres MEALPLAN_TEST_DSN='postgres://user:pass@localhost:5432/db?sslmode=disable' go test ./...`.
3. Zmiana API wymaga aktualizacji `api/openapi.yaml` (test to egzekwuje).
4. SQL musi być przenośny między SQLite i Postgres (znaki `?`, bez składni specyficznej dla jednego silnika).
5. Diagramy w dokumentacji piszemy w mermaid.
6. Commity w formie `feat:`, `fix:`, `docs:`, `test:`.

```mermaid
flowchart LR
    A[Zmiana kodu] --> B[Testy Go i web]
    B --> C[Aktualizacja openapi.yaml i docs]
    C --> D[Pull request]
```
````

`CHANGELOG.md`:

```markdown
# Changelog

## Unreleased

- Etap 1: konfiguracja, baza z migracjami (SQLite, pod Postgres), logowanie argon2id, sesje, CSRF, limity prób, zaproszenia, API z OpenAPI, powłoka UI Vue + Vuetify, osadzony frontend, Dockerfile, CI.
```

- [ ] **Step 6: Pipeline CI**

`.github/workflows/ci.yml`:

```yaml
name: ci
on:
  push:
    branches: [main]
  pull_request:

jobs:
  go:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        db: [sqlite, postgres]
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_PASSWORD: mealplan
          POSTGRES_DB: mealplan
        ports: ["5432:5432"]
        options: >-
          --health-cmd "pg_isready -U postgres"
          --health-interval 5s --health-timeout 5s --health-retries 10
    env:
      MEALPLAN_TEST_DRIVER: ${{ matrix.db }}
      MEALPLAN_TEST_DSN: postgres://postgres:mealplan@localhost:5432/mealplan?sslmode=disable
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.23" }
      - name: gofmt
        run: test -z "$(gofmt -l .)"
      - run: go vet ./...
      - run: go test ./... -race

  web:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: "22", cache: npm, cache-dependency-path: web/package-lock.json }
      - run: npm ci
      - run: npm test
      - run: npm run build
      - run: npm audit --omit=dev --audit-level=high

  vuln:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.23" }
      - run: go install golang.org/x/vuln/cmd/govulncheck@latest
      - run: govulncheck ./...

  docker:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: docker build -t mealplan:ci .
```

- [ ] **Step 7: Uruchom pełny zestaw testów lokalnie**

Run: `gofmt -l . ; go vet ./... && go test ./... -race && (cd web && npm test && npm run build)`
Expected: brak wyjścia z `gofmt -l` (po wcześniejszych `gofmt -w .`); wszystkie pakiety Go `ok` (w tym `docs`); testy Vitest PASS; build bez błędów.

Jeśli masz lokalny Postgres, uruchom dodatkowo macierz Postgres:

```bash
MEALPLAN_TEST_DRIVER=postgres MEALPLAN_TEST_DSN='postgres://postgres:mealplan@localhost:5432/mealplan?sslmode=disable' go test ./... -race
```

Expected: PASS na obu silnikach. Pominięcie tego kroku zgłoś w opisie commita (CI sprawdzi Postgres).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "docs: stage 1 documentation and CI pipeline for SQLite and Postgres"
```

---

## Self-Review (wykonany przy pisaniu planu)

**Pokrycie specu (zakres Etapu 1):**
- Szkielet repo, MIT, konfiguracja env: Task 1.
- Baza, migracje, SQLite + Postgres przez wspólne repozytoria i `Rebind`: Task 2 i 4; macierz CI: Task 9.
- argon2id, sesje, CSRF, limity prób: Task 3, 5, 6.
- Gospodarstwa, role, zaproszenia (utworzenie, lista, unieważnienie, podgląd, przyjęcie): Task 4 do 6.
- Izolacja `household_id` z testami: Task 4, 5, 6.
- OpenAPI jako źródło prawdy z testem zgodności: Task 6.
- UI Vue + Vuetify (Material, jasny/ciemny, mobile/desktop): Task 7.
- `go:embed`, jeden plik binarny, Docker: Task 8.
- CI, dokumentacja z mermaid, test diagramów: Task 9.

**Świadome odstępstwa od specu (zapisane w planie i dokumentacji):**
- Ścieżki zaproszeń to `/auth/invitations/{token}` (OpenAPI nie dopuszcza dwóch ścieżek różniących się tylko nazwą parametru). Spec jest korygowany w Task 6, Step 8.
- CSP: `style-src 'unsafe-inline'` tymczasowo (Vuetify), skrypty ścisłe. Opisane w `docs/security.md` i w specu.
- Limiter prób w pamięci procesu (spec wymaga bezstanowości dla `hosted`); przeniesienie do bazy w Etapie 5. Opisane w `docs/security.md`.
- Zmiana roli i usuwanie członków (spec, sekcja 5) przeniesione na Etap 2; w Etapie 1 jest listowanie członków.
- Testy Playwright i PWA poza Etapem 1 (spec: Etap 4 dla PWA); w Etapie 1 weryfikacja ręczna (Task 8, Step 7 i 8) oraz Vitest dla logiki frontendu.

**Spójność typów:** `Principal` ma pole `Login` od Task 5 (używane przez `handleMe` w Task 6); `store.ErrNotFound` zwracany bez opakowania przez `RevokeInvite` i mapowany w `fail`; `SessionGrant` używany identycznie w `auth` i `httpapi`.
