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
