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
		"mode":           {"MEALPLAN_MODE": "cloud"},
		"driver":         {"MEALPLAN_DB_DRIVER": "mysql"},
		"bool":           {"MEALPLAN_COOKIE_SECURE": "maybe"},
		"duration":       {"MEALPLAN_SESSION_IDLE": "soon"},
		"max below idle": {"MEALPLAN_SESSION_IDLE": "48h", "MEALPLAN_SESSION_MAX": "24h"},
	}
	for name, m := range cases {
		if _, err := Load(env(m)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
