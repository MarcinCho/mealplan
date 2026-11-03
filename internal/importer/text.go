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
	text = strings.TrimPrefix(text, "\uFEFF")
	text = strings.NewReplacer("\r\n", "\n", "\r", "\n", "\u00a0", " ").Replace(text)
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
		// Numbering ("1.", "2)") is list decoration, not a quantity; Raw keeps the original line.
		p := ParseIngredient(stepMarker.ReplaceAllString(l, ""))
		p.Raw = l
		d.Ingredients = append(d.Ingredients, p)
	}
	d.Steps = buildSteps(stepRaw)
	return d
}
