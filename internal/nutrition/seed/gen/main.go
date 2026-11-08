// Command gen builds products.csv from the USDA FoodData Central "SR Legacy"
// CSV release and the hand-written mapping.csv.
//
//	go run ./internal/nutrition/seed/gen -fdc <dir with food.csv, food_nutrient.csv, food_portion.csv> \
//	  -mapping internal/nutrition/seed/mapping.csv -out internal/nutrition/seed/products.csv
//
// Nutrient values are never typed by hand: they come from food_nutrient.csv.
// Optional piece weights and densities are either literal numbers or are
// derived from food_portion.csv of the same food (see mapping.csv header).
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mealplan/internal/units"
)

const (
	nutKcal    = "1008"
	nutProtein = "1003"
	nutFat     = "1004"
	nutCarbs   = "1005"

	mlCup  = 236.5882365
	mlTbsp = 14.78676478
	mlTsp  = 4.92892159
)

type portion struct {
	amount float64
	text   string
	grams  float64
}

func main() {
	fdc := flag.String("fdc", "", "directory with the unpacked FDC SR Legacy CSV files")
	mapping := flag.String("mapping", "", "mapping.csv")
	out := flag.String("out", "", "products.csv to write")
	release := flag.String("release", "April 2018", "SR Legacy release date for the header")
	flag.Parse()
	if *fdc == "" || *mapping == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*fdc, *mapping, *out, *release); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func readCSV(path string, comma rune) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = comma
	r.Comment = '#'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var rows [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		rows = append(rows, rec)
	}
}

func run(fdc, mappingPath, outPath, release string) error {
	foods, err := readCSV(filepath.Join(fdc, "food.csv"), ',')
	if err != nil {
		return err
	}
	desc := map[string]string{}
	for _, r := range foods[1:] {
		desc[r[0]] = r[2]
	}

	nuts, err := readCSV(filepath.Join(fdc, "food_nutrient.csv"), ',')
	if err != nil {
		return err
	}
	// fdc_id -> nutrient_id -> amount per 100 g
	values := map[string]map[string]float64{}
	for _, r := range nuts[1:] {
		switch r[2] {
		case nutKcal, nutProtein, nutFat, nutCarbs:
			v, err := strconv.ParseFloat(r[3], 64)
			if err != nil {
				return fmt.Errorf("food_nutrient %s: %w", r[0], err)
			}
			if values[r[1]] == nil {
				values[r[1]] = map[string]float64{}
			}
			values[r[1]][r[2]] = v
		}
	}

	ports, err := readCSV(filepath.Join(fdc, "food_portion.csv"), ',')
	if err != nil {
		return err
	}
	portions := map[string][]portion{}
	for _, r := range ports[1:] {
		amount, err1 := strconv.ParseFloat(r[3], 64)
		grams, err2 := strconv.ParseFloat(r[7], 64)
		if err1 != nil || err2 != nil || amount <= 0 {
			continue
		}
		text := strings.TrimSpace(strings.ToLower(r[5] + " " + r[6]))
		portions[r[1]] = append(portions[r[1]], portion{amount, text, grams})
	}

	rows, err := readCSV(mappingPath, ';')
	if err != nil {
		return err
	}
	if len(rows) == 0 || strings.Join(rows[0], ";") != "name;aliases;fdc_id;piece_grams;density" {
		return fmt.Errorf("%s: unexpected header", mappingPath)
	}

	var sb strings.Builder
	sb.WriteString("# GENERATED FILE - do not edit. Regenerate with:\n")
	sb.WriteString("#   go run ./internal/nutrition/seed/gen -fdc <dir> -mapping internal/nutrition/seed/mapping.csv -out internal/nutrition/seed/products.csv\n")
	fmt.Fprintf(&sb, "# Source: USDA FoodData Central, SR Legacy (%s release), https://fdc.nal.usda.gov/download-datasets\n", release)
	sb.WriteString("# Licence: CC0 1.0 Universal (public domain), https://creativecommons.org/publicdomain/zero/1.0/\n")
	sb.WriteString("# Macros are grams (kcal for energy) per 100 g edible portion; piece_grams in g; density in g/ml.\n")
	sb.WriteString("name;aliases;kcal;protein;carbs;fat;piece_grams;density_g_ml\n")

	var problems []string
	for _, r := range rows[1:] {
		if len(r) != 5 {
			return fmt.Errorf("mapping row %q: want 5 columns, got %d", r, len(r))
		}
		name := units.NormalizeName(r[0])
		id := strings.TrimSpace(r[2])
		v, ok := values[id]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: fdc_id %s has no nutrients", name, id))
			continue
		}
		var missing []string
		for _, n := range []string{nutKcal, nutProtein, nutFat, nutCarbs} {
			if _, ok := v[n]; !ok {
				missing = append(missing, n)
			}
		}
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("%s: fdc_id %s (%s) lacks nutrient ids %v", name, id, desc[id], missing))
			continue
		}
		piece, err := resolvePiece(r[3], portions[id])
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: piece_grams: %v", name, err))
			continue
		}
		density, err := resolveDensity(r[4], portions[id])
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: density: %v", name, err))
			continue
		}
		var aliases []string
		for _, a := range strings.Split(r[1], "|") {
			if a = units.NormalizeName(a); a != "" {
				aliases = append(aliases, a)
			}
		}
		fmt.Fprintf(&sb, "%s;%s;%s;%s;%s;%s;%s;%s\n", name, strings.Join(aliases, "|"),
			num(v[nutKcal]), num(v[nutProtein]), num(v[nutCarbs]), num(v[nutFat]), piece, density)
		fmt.Fprintf(os.Stderr, "%-28s <- %s %s\n", name, id, desc[id])
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problems:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
	return os.WriteFile(outPath, []byte(sb.String()), 0o644)
}

func num(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// find returns the portion whose text equals q, else the first containing q.
func find(ps []portion, q string) (portion, bool) {
	q = strings.ToLower(strings.TrimSpace(q))
	for _, p := range ps {
		if p.text == q {
			return p, true
		}
	}
	for _, p := range ps {
		if strings.Contains(p.text, q) {
			return p, true
		}
	}
	return portion{}, false
}

// resolvePiece: "" none; "p=<text>" grams per one portion row; otherwise a literal number.
func resolvePiece(cell string, ps []portion) (string, error) {
	cell = strings.TrimSpace(cell)
	switch {
	case cell == "":
		return "", nil
	case strings.HasPrefix(cell, "p="):
		p, ok := find(ps, cell[2:])
		if !ok {
			return "", fmt.Errorf("no food_portion row matching %q", cell[2:])
		}
		return num(round(p.grams/p.amount, 1)), nil
	}
	if _, err := strconv.ParseFloat(cell, 64); err != nil {
		return "", err
	}
	return cell, nil
}

// resolveDensity: "" none; "cup=<text>", "tbsp=<text>", "tsp=<text>" derive g/ml from a
// food_portion row measured in that volume unit; otherwise a literal number.
func resolveDensity(cell string, ps []portion) (string, error) {
	cell = strings.TrimSpace(cell)
	if cell == "" {
		return "", nil
	}
	for prefix, ml := range map[string]float64{"cup=": mlCup, "tbsp=": mlTbsp, "tsp=": mlTsp} {
		if strings.HasPrefix(cell, prefix) {
			p, ok := find(ps, cell[len(prefix):])
			if !ok {
				return "", fmt.Errorf("no food_portion row matching %q", cell[len(prefix):])
			}
			return num(round(p.grams/(p.amount*ml), 3)), nil
		}
	}
	if _, err := strconv.ParseFloat(cell, 64); err != nil {
		return "", err
	}
	return cell, nil
}

func round(f float64, digits int) float64 {
	p := 1.0
	for i := 0; i < digits; i++ {
		p *= 10
	}
	return float64(int64(f*p+0.5)) / p
}
