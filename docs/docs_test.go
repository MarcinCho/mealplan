package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var diagramTypes = []string{"flowchart", "sequenceDiagram", "erDiagram", "stateDiagram-v2", "stateDiagram", "classDiagram", "graph"}

// Every ```mermaid fence in the documentation must be closed and start with a known diagram type.
func TestMermaidBlocksAreWellFormed(t *testing.T) {
	files, err := filepath.Glob("*.md")
	if err != nil {
		t.Fatal(err)
	}
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
			fields := strings.Fields(first)
			for _, d := range diagramTypes {
				if len(fields) > 0 && fields[0] == d {
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
