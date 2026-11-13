package assistant

import (
	"sort"
	"strings"

	"mealplan/internal/domain"
	"mealplan/internal/nutrition"
	"mealplan/internal/store"
	"mealplan/internal/units"
)

// IngredientCandidate is one ingredient name worth asking the model about, with every recipe
// that uses it. CurrentProductID is set when the ingredient is linked to a product whose name
// and aliases do not contain the ingredient's name (a suspicious link).
type IngredientCandidate struct {
	Name, NormalizedName string
	CurrentProductID     string
	RecipeIDs            []string
	RecipeTitles         []string
}

// CandidateIngredients picks the ingredients without a product that do not match a product
// exactly, and the ingredients linked to a product that does not carry their name. They are
// grouped by normalized name and current product and sorted by name.
func CandidateIngredients(refs []store.IngredientRef, products []domain.Product) []IngredientCandidate {
	m := nutrition.NewMatcher(products)
	names := make(map[string]map[string]bool, len(products)) // product id -> its normalized names
	for _, p := range products {
		set := map[string]bool{p.NormalizedName: true, units.NormalizeName(p.Name): true}
		for _, a := range p.Aliases {
			set[units.NormalizeName(a)] = true
		}
		names[p.ID] = set
	}
	type groupKey struct{ name, product string }
	groups := map[groupKey]*IngredientCandidate{}
	out := []IngredientCandidate{}
	var order []groupKey
	for _, ref := range refs {
		key := units.NormalizeName(ref.Name)
		if key == "" {
			key = ref.NormalizedName
		}
		if key == "" {
			continue
		}
		// A link to a product that no longer exists counts as no link.
		current := ""
		if own, ok := names[ref.ProductID]; ok && ref.ProductID != "" {
			if own[key] {
				continue
			}
			current = ref.ProductID
		} else if _, ok := m.Match(domain.Ingredient{Name: ref.Name, NormalizedName: key}); ok {
			continue
		}
		gk := groupKey{key, current}
		c := groups[gk]
		if c == nil {
			c = &IngredientCandidate{Name: strings.TrimSpace(ref.Name), NormalizedName: key, CurrentProductID: current}
			groups[gk] = c
			order = append(order, gk)
		}
		if !contains(c.RecipeIDs, ref.RecipeID) {
			c.RecipeIDs = append(c.RecipeIDs, ref.RecipeID)
			c.RecipeTitles = append(c.RecipeTitles, ref.RecipeTitle)
		}
	}
	for _, gk := range order {
		out = append(out, *groups[gk])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].NormalizedName != out[j].NormalizedName {
			return out[i].NormalizedName < out[j].NormalizedName
		}
		return out[i].CurrentProductID < out[j].CurrentProductID
	})
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// CandidateProductBatches groups products that might be duplicates and packs the groups into
// batches of at most max ids. Products are sorted by normalized name and grouped by the first
// three letters of their first word; products with no such neighbour are left out. A group
// larger than max is split over consecutive batches; a lone remainder shares the batch with the
// previous chunk's last id, so the products of a group are never left out.
func CandidateProductBatches(products []domain.Product, max int) [][]string {
	out := [][]string{}
	if max < 2 {
		return out
	}
	type entry struct{ id, name string }
	list := make([]entry, 0, len(products))
	for _, p := range products {
		list = append(list, entry{p.ID, units.NormalizeName(p.Name)})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].name != list[j].name {
			return list[i].name < list[j].name
		}
		return list[i].id < list[j].id
	})
	var cur []string
	flush := func() {
		if len(cur) > 1 {
			out = append(out, cur)
		}
		cur = nil
	}
	add := func(group []string) {
		if len(cur)+len(group) > max {
			flush()
		}
		if len(group) > max {
			for len(group) > max {
				out = append(out, group[:max:max])
				group = group[max:]
			}
			// The chunks above are full, so a lone remainder cannot join the last one: it is
			// paired with that chunk's last id instead, so no product of the group is left out.
			if len(group) == 1 {
				prev := out[len(out)-1]
				group = []string{prev[len(prev)-1], group[0]}
			}
		}
		cur = append(cur, group...)
	}
	for i := 0; i < len(list); {
		k := stem(list[i].name)
		j := i + 1
		for j < len(list) && stem(list[j].name) == k {
			j++
		}
		if j-i > 1 && k != "" {
			group := make([]string, 0, j-i)
			for _, e := range list[i:j] {
				group = append(group, e.id)
			}
			add(group)
		}
		i = j
	}
	flush()
	return out
}

// stem is the first three letters of the first word (the whole word if it is shorter), so that
// "ser" and "sery" or "ryż" and "ryżu" land in one group.
func stem(name string) string {
	f := strings.Fields(name)
	if len(f) == 0 {
		return ""
	}
	r := []rune(f[0])
	if len(r) > 3 {
		r = r[:3]
	}
	return string(r)
}
