package importer

import (
	"bytes"
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

var (
	tagRe    = regexp.MustCompile(`<[^>]*>`)
	breakRe  = regexp.MustCompile(`(?i)<\s*(?:br\s*/?|/p|/li|/div)\s*>`)
	firstNum = regexp.MustCompile(`\d+`)
	// "około 1650 ml" is a volume, not a number of servings.
	volumeYield = regexp.MustCompile(`(?i)\d\s*(?:ml|l|litr\w*|g|kg|dag)\b`)
)

var skipTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "nav": true, "footer": true, "header": true,
	"aside": true, "form": true, "svg": true, "template": true, "iframe": true,
}

var blockTags = map[string]bool{
	"p": true, "div": true, "li": true, "br": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true, "tr": true, "ul": true, "ol": true, "section": true, "article": true,
	"table": true, "dt": true, "dd": true,
}

// ParseHTML extracts a recipe from a web page: JSON-LD first, then microdata, then the visible text.
func ParseHTML(body []byte) (Draft, error) {
	doc, err := xhtml.Parse(bytes.NewReader(body))
	if err != nil {
		return Draft{}, ErrUnparsable
	}
	if d, ok := fromJSONLD(doc); ok && usable(d) {
		return d, nil
	}
	if d, ok := fromMicrodata(doc); ok && usable(d) {
		return d, nil
	}
	return fromPageText(doc)
}

func usable(d Draft) bool { return len(d.Ingredients)+len(d.Steps) > 0 }

func walk(n *xhtml.Node, f func(*xhtml.Node) bool) {
	if !f(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, f)
	}
}

func attr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *xhtml.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

// plain strips markup and entities and collapses whitespace.
func plain(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(tagRe.ReplaceAllString(s, " "))), " ")
}

func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(breakRe.ReplaceAllString(s, "\n"), "\n") {
		if l = plain(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// blockText returns the visible text under n with a newline around block elements.
func blockText(n *xhtml.Node) string {
	var b strings.Builder
	var rec func(*xhtml.Node)
	rec = func(n *xhtml.Node) {
		switch n.Type {
		case xhtml.TextNode:
			b.WriteString(n.Data)
		case xhtml.ElementNode:
			if skipTags[n.Data] {
				return
			}
			block := blockTags[n.Data]
			if block {
				b.WriteByte('\n')
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				rec(c)
			}
			if block {
				b.WriteByte('\n')
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(n)
	return b.String()
}

// capDraft bounds the lists a structured source can hold and says so.
func capDraft(d *Draft) {
	if len(d.Ingredients) > parseMaxIng {
		d.Ingredients = d.Ingredients[:parseMaxIng]
		d.warn("Pominięto składniki ponad limit %d.", maxIngredients)
	}
	if len(d.Steps) > parseMaxSteps {
		d.Steps = d.Steps[:parseMaxSteps]
		d.warn("Pominięto kroki ponad limit %d.", maxSteps)
	}
}

func yieldOf(s string) int {
	if volumeYield.MatchString(s) {
		return 0
	}
	if m := firstNum.FindString(s); m != "" {
		n, _ := strconv.Atoi(m)
		return n
	}
	return 0
}

func ingredientLines(raw []string) []IngredientLine {
	var out []IngredientLine
	for _, r := range raw {
		for _, l := range lines(r) {
			if len(out) > parseMaxIng {
				return out
			}
			out = append(out, ParseIngredient(l))
		}
	}
	return out
}

// --- JSON-LD ---

func fromJSONLD(doc *xhtml.Node) (Draft, bool) {
	var cands []map[string]any
	walk(doc, func(n *xhtml.Node) bool {
		if n.Type == xhtml.ElementNode && n.Data == "script" && strings.EqualFold(attr(n, "type"), "application/ld+json") {
			if n.FirstChild != nil {
				var v any
				raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(n.FirstChild.Data), "\ufeff"))
				// Valid JSON is used as is; stripping wrapper markers would also delete them inside string values.
				err := json.Unmarshal([]byte(raw), &v)
				if err != nil {
					err = json.Unmarshal([]byte(cleanJSONLD(raw)), &v)
				}
				if err == nil {
					cands = findRecipes(v, cands)
				}
			}
			return false
		}
		return true
	})
	if len(cands) == 0 {
		return Draft{}, false
	}
	first := draftFromLD(cands[0])
	for i, c := range cands {
		d := first
		if i > 0 {
			d = draftFromLD(c)
		}
		if usable(d) {
			return d, true
		}
	}
	return first, true
}

var ldMarkers = strings.NewReplacer("<!--", "", "-->", "", "//<![CDATA[", "", "<![CDATA[", "", "//]]>", "", "]]>", "")

// cleanJSONLD removes a BOM and the comment/CDATA wrappers some sites put around JSON-LD.
func cleanJSONLD(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "\ufeff"))
	return strings.TrimSpace(ldMarkers.Replace(s))
}

func isRecipeType(t any) bool {
	switch x := t.(type) {
	case string:
		return x == "Recipe" || strings.HasSuffix(x, "/Recipe")
	case []any:
		for _, e := range x {
			if isRecipeType(e) {
				return true
			}
		}
	}
	return false
}

// findRecipes appends every Recipe node reachable from v (top-level arrays, @graph, mainEntity).
func findRecipes(v any, out []map[string]any) []map[string]any {
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			out = findRecipes(e, out)
		}
	case map[string]any:
		if isRecipeType(x["@type"]) {
			return append(out, x)
		}
		for _, k := range []string{"@graph", "mainEntity", "mainEntityOfPage"} {
			out = findRecipes(x[k], out)
		}
	}
	return out
}

func ldString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		for _, e := range x {
			if s := ldString(e); s != "" {
				return s
			}
		}
	}
	return ""
}

func ldStrings(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		var out []string
		for _, e := range x {
			if s := ldString(e); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func ldYield(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		return yieldOf(x)
	case []any:
		for _, e := range x {
			if n := ldYield(e); n > 0 {
				return n
			}
		}
	}
	return 0
}

func ldInstructions(v any) []string {
	switch x := v.(type) {
	case string:
		return lines(x)
	case []any:
		var out []string
		for _, e := range x {
			out = append(out, ldInstructions(e)...)
		}
		return out
	case map[string]any:
		if list, ok := x["itemListElement"]; ok {
			return ldInstructions(list)
		}
		if t := ldString(x["text"]); t != "" {
			return lines(t)
		}
		return lines(ldString(x["name"]))
	}
	return nil
}

func ldTags(v any) []string {
	var out []string
	for _, s := range ldStrings(v) {
		for _, p := range strings.Split(s, ",") {
			if p = plain(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func draftFromLD(m map[string]any) Draft {
	var d Draft
	d.Title = plain(ldString(m["name"]))
	d.Servings = ldYield(m["recipeYield"])
	raw := ldStrings(m["recipeIngredient"])
	if len(raw) == 0 {
		raw = ldStrings(m["ingredients"])
	}
	d.Ingredients = ingredientLines(raw)
	d.Steps = ldInstructions(m["recipeInstructions"])
	d.Tags = append(ldTags(m["keywords"]), ldTags(m["recipeCategory"])...)
	capDraft(&d)
	return d
}

// --- microdata ---

func propValue(n *xhtml.Node) string {
	switch n.Data {
	case "meta":
		return attr(n, "content")
	case "time":
		if v := attr(n, "datetime"); v != "" {
			return v
		}
	}
	return blockText(n)
}

func fromMicrodata(doc *xhtml.Node) (Draft, bool) {
	var scope *xhtml.Node
	walk(doc, func(n *xhtml.Node) bool {
		if scope != nil {
			return false
		}
		if n.Type == xhtml.ElementNode && strings.HasSuffix(strings.TrimRight(attr(n, "itemtype"), "/"), "/Recipe") {
			scope = n
			return false
		}
		return true
	})
	if scope == nil {
		return Draft{}, false
	}
	props := map[string][]string{}
	var collect func(*xhtml.Node)
	collect = func(n *xhtml.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != xhtml.ElementNode {
				continue
			}
			if p := attr(c, "itemprop"); p != "" {
				for _, name := range strings.Fields(p) {
					props[name] = append(props[name], propValue(c))
				}
				continue
			}
			// A nested item without an itemprop (breadcrumbs, ratings) owns its properties.
			if hasAttr(c, "itemscope") {
				continue
			}
			collect(c)
		}
	}
	collect(scope)

	var d Draft
	if v := props["name"]; len(v) > 0 {
		d.Title = plain(v[0])
	}
	if v := props["recipeYield"]; len(v) > 0 {
		d.Servings = yieldOf(v[0])
	}
	d.Ingredients = ingredientLines(append(props["recipeIngredient"], props["ingredients"]...))
	for _, s := range props["recipeInstructions"] {
		d.Steps = append(d.Steps, lines(s)...)
	}
	for _, s := range append(props["keywords"], props["recipeCategory"]...) {
		for _, p := range strings.Split(s, ",") {
			if p = plain(p); p != "" {
				d.Tags = append(d.Tags, p)
			}
		}
	}
	capDraft(&d)
	return d, true
}

// --- visible text ---

func pageTitle(doc *xhtml.Node) string {
	var h1, title string
	walk(doc, func(n *xhtml.Node) bool {
		if n.Type == xhtml.ElementNode {
			switch n.Data {
			case "h1":
				if h1 == "" {
					h1 = plain(blockText(n))
				}
			case "title":
				if title == "" && n.FirstChild != nil {
					title = plain(n.FirstChild.Data)
				}
			}
		}
		return true
	})
	if h1 != "" {
		return h1
	}
	return title
}

// fromPageText is the last resort. It only accepts pages that clearly contain both parts of a recipe,
// otherwise navigation and prose would turn into a bogus recipe.
func fromPageText(doc *xhtml.Node) (Draft, error) {
	d := ParseText(blockText(doc))
	if len(d.Ingredients) < 2 || len(d.Steps) < 1 {
		return Draft{}, ErrUnparsable
	}
	if t := pageTitle(doc); t != "" {
		d.Title = t
	}
	d.Warnings = append([]string{"Dane strukturalne nie zostały znalezione, wynik opiera się na tekście strony. Sprawdź go dokładnie."}, d.Warnings...)
	return d, nil
}
