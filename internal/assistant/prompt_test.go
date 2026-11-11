package assistant

import (
	"encoding/json"
	"strings"
	"testing"

	"mealplan/internal/product"
	"mealplan/internal/store"
)

// The real store and product service satisfy the interfaces the service consumes.
var (
	_ SettingsRepo  = (*store.Store)(nil)
	_ ProductLister = (*product.Service)(nil)
)

func TestPromptRules(t *testing.T) {
	for name, text := range map[string]string{"match": matchSystem, "duplicates": duplicatesSystem} {
		for _, must := range []string{`"czerwona cebula" ≠ "cebula"`, `"pomidory" = "pomidor"`, "mianowniku liczby pojedynczej", "WYŁĄCZNIE id z listy"} {
			if !strings.Contains(text, must) {
				t.Errorf("%s prompt lacks %q", name, must)
			}
		}
	}
	for _, must := range []string{`"none"`, "na 100 g SUROWEGO produktu"} {
		if !strings.Contains(matchSystem, must) {
			t.Errorf("match prompt lacks %q", must)
		}
	}
	for name, s := range map[string]json.RawMessage{"match": matchSchema, "duplicates": duplicatesSchema, "test": testSchema} {
		if !json.Valid(s) {
			t.Errorf("%s schema is not valid JSON", name)
		}
	}
}
