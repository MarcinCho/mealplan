package docs

import (
	"os"
	"path/filepath"
	"regexp"
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

var mdLink = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

// Every relative link in the top-level and docs Markdown files must point to an existing file.
func TestRelativeLinksResolve(t *testing.T) {
	files, err := filepath.Glob("*.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"README.md", "CONTRIBUTING.md", "SECURITY.md", "CODE_OF_CONDUCT.md"} {
		files = append(files, filepath.Join("..", f))
	}
	checked := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		inFence := false
		for n, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			for _, m := range mdLink.FindAllStringSubmatch(line, -1) {
				target := m[1]
				if strings.HasPrefix(target, "#") || strings.HasPrefix(target, "http://") ||
					strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "mailto:") {
					continue
				}
				if i := strings.Index(target, "#"); i >= 0 {
					target = target[:i]
				}
				checked++
				if _, err := os.Stat(filepath.Join(filepath.Dir(f), target)); err != nil {
					t.Errorf("%s:%d: broken relative link %q", f, n+1, m[1])
				}
			}
		}
	}
	if checked < 10 {
		t.Fatalf("expected to check at least 10 relative links, checked %d", checked)
	}
}
