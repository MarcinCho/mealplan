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
