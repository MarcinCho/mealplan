package importer

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"mealplan/internal/units"
)

var knownUnits = map[string]bool{
	"g": true, "kg": true, "ml": true, "l": true, "szt": true,
	"łyżka": true, "łyżeczka": true, "szklanka": true, "ząbek": true, "plaster": true,
	"szczypta": true, "opakowanie": true, "puszka": true, "pęczek": true, "garść": true,
	"kostka": true, "główka": true, "kromka": true,
}

// Adjectives that sit between an amount and its unit ("1 płaska łyżeczka", "1 duży ząbek").
var fillers = map[string]bool{
	"płaska": true, "płaską": true, "płaskie": true, "czubata": true, "czubatą": true, "czubate": true,
	"kopiasta": true, "gładka": true, "duża": true, "duży": true, "małe": true, "mała": true, "mały": true,
	"średnia": true, "średni": true, "średnie": true, "duże": true, "dużą": true, "małą": true,
	"kopiaste": true, "kopiastą": true, "średnią": true, "gładkie": true,
}

// takeUnitAfterFiller is takeUnit that also skips one filler adjective when a known unit follows it.
func takeUnitAfterFiller(tk []string) (unit string, mult float64, n int) {
	if len(tk) > 1 && fillers[strings.ToLower(tk[0])] {
		if u, m, un := takeUnit(tk[1:]); un > 0 {
			return u, m, un + 1
		}
	}
	return takeUnit(tk)
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

func hasLetterOrDigit(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func hasDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// trailingAmount finds the first "<qty> <unit>" after at least one name token ("czosnek 2 ząbki"). The unit is
// required so "mąka typ 500" and "sos 1000 wysp" stay as they are. Whatever follows must be empty or note-like
// (another amount, "po 150 g", "- około 200 g"); it is returned as the note.
func trailingAmount(tk []string) (v float64, unit string, at int, note string, ok bool) {
	for i := 1; i < len(tk); i++ {
		q, n, _, qok := parseQuantity(tk[i:])
		if !qok {
			continue
		}
		u, mult, un := takeUnitAfterFiller(tk[i+n:])
		if un == 0 {
			continue
		}
		tail := tk[i+n+un:]
		if len(tail) == 0 {
			return q * mult, u, i, "", true
		}
		if noteLikeTail(tail) {
			return q * mult, u, i, strings.Trim(strings.Join(tail, " "), " -–—"), true
		}
	}
	return 0, "", 0, "", false
}

func noteLikeTail(tail []string) bool {
	switch strings.ToLower(tail[0]) {
	case "po", "-", "–", "—", "około", "ok.":
		return true
	}
	if _, n, _, ok := parseQuantity(tail); ok {
		_, _, un := takeUnitAfterFiller(tail[n:])
		return un > 0
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
		u, mult, un := takeUnitAfterFiller(tk[n:])
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
	} else if v, u, i, tn, ok := trailingAmount(tk); ok {
		qty, unit, rest, guess, consumed = &v, u, tk[:i], true, true
		if tn != "" {
			if note == "" {
				note = tn
			} else {
				note = tn + ", " + note
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
	case name == "" || !hasLetterOrDigit(name) || utf8.RuneCountInString(name) > maxTitle:
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
