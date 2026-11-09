package importer

import (
	"math"
	"os"
	"strings"
	"testing"
)

// The examples in the downloadable Claude skill must import cleanly and carry consistent values,
// because Claude copies their shape.
func TestSkillExamplesImport(t *testing.T) {
	raw, err := os.ReadFile("../../web/public/claude-skill/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "Output:\n\n```\n")
	if len(parts) < 3 {
		t.Fatalf("want at least 2 output examples, got %d", len(parts)-1)
	}
	for i, p := range parts[1:] {
		block, _, ok := strings.Cut(p, "\n```")
		if !ok {
			t.Fatalf("example %d: unterminated block", i)
		}
		d := ParseText(block)
		if d.Title == "" || d.Servings == 0 || len(d.Ingredients) == 0 || len(d.Steps) == 0 || len(d.Warnings) > 0 {
			t.Fatalf("example %d did not import cleanly: %+v", i, d)
		}
		n := d.Nutrition
		if n.Kcal == nil || n.Protein == nil || n.Carbs == nil || n.Fat == nil {
			t.Fatalf("example %d: missing nutrition values %+v", i, n)
		}
		est := 4**n.Protein + 4**n.Carbs + 9**n.Fat
		if math.Abs(est-*n.Kcal) > 0.1**n.Kcal {
			t.Fatalf("example %d: macros give %.0f kcal, line says %.0f", i, est, *n.Kcal)
		}
	}
}
