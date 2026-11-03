package importer

import "testing"

func f(v float64) *float64 { return &v }

func TestParseIngredient(t *testing.T) {
	cases := []struct {
		raw  string
		name string
		qty  *float64
		unit string
		note string
		conf string
	}{
		{"200 g mąki", "mąki", f(200), "g", "", ConfOK},
		{"1,5 kg ziemniaków", "ziemniaków", f(1.5), "kg", "", ConfOK},
		{"1.5 łyżeczki soli", "soli", f(1.5), "łyżeczka", "", ConfOK},
		{"0,5 kg mięsa mielonego", "mięsa mielonego", f(0.5), "kg", "", ConfOK},
		{"1/2 szklanki mleka", "mleka", f(0.5), "szklanka", "", ConfOK},
		{"3/4 szklanki cukru", "cukru", f(0.75), "szklanka", "", ConfOK},
		{"1 ½ szklanki mąki", "mąki", f(1.5), "szklanka", "", ConfOK},
		{"1 1/2 szklanki mąki", "mąki", f(1.5), "szklanka", "", ConfOK},
		{"½ łyżeczki soli", "soli", f(0.5), "łyżeczka", "", ConfOK},
		{"¾ szklanki cukru", "cukru", f(0.75), "szklanka", "", ConfOK},
		{"półtora szklanki mąki", "mąki", f(1.5), "szklanka", "", ConfOK},
		{"pół litra mleka", "mleka", f(0.5), "l", "", ConfOK},
		{"ćwierć łyżeczki soli", "soli", f(0.25), "łyżeczka", "", ConfOK},
		{"2-3 ząbki czosnku", "czosnku", f(3), "ząbek", "", ConfGuess},
		{"1-2 łyżki oleju", "oleju", f(2), "łyżka", "", ConfGuess},
		{"2–3 jabłka", "jabłka", f(3), "szt", "", ConfGuess},
		{"2 do 3 łyżek oleju", "oleju", f(3), "łyżka", "", ConfGuess},
		{"szczypta soli", "soli", nil, "szczypta", "", ConfOK},
		{"garść orzechów włoskich", "orzechów włoskich", nil, "garść", "", ConfOK},
		{"sól", "sól", nil, "", "", ConfOK},
		{"Sól", "Sól", nil, "", "", ConfOK},
		{"sól do smaku", "sól", nil, "", "do smaku", ConfOK},
		{"pieprz, do smaku", "pieprz", nil, "", "do smaku", ConfOK},
		{"3 jajka", "jajka", f(3), "szt", "", ConfGuess},
		{"4 ziemniaki średnie", "ziemniaki średnie", f(4), "szt", "", ConfGuess},
		{"3 szt. jajek", "jajek", f(3), "szt", "", ConfOK},
		{"250g masła", "masła", f(250), "g", "", ConfOK},
		{"2łyżki cukru", "cukru", f(2), "łyżka", "", ConfOK},
		{"1 cebula, posiekana", "cebula", f(1), "szt", "posiekana", ConfGuess},
		{"100 g masła, rozpuszczonego", "masła", f(100), "g", "rozpuszczonego", ConfOK},
		{"1 puszka pomidorów (400 g)", "pomidorów", f(1), "puszka", "400 g", ConfOK},
		{"1 szklanka (250 ml) mleka", "mleka", f(1), "szklanka", "250 ml", ConfOK},
		{"1 opakowanie drożdży (7 g)", "drożdży", f(1), "opakowanie", "7 g", ConfOK},
		{"500 ml mleka (2%)", "mleka", f(500), "ml", "2%", ConfOK},
		{"200 g cukru do posypania", "cukru", f(200), "g", "do posypania", ConfOK},
		{"2 łyżki oliwy z oliwek", "oliwy z oliwek", f(2), "łyżka", "", ConfOK},
		{"1 łyżeczka proszku do pieczenia", "proszku do pieczenia", f(1), "łyżeczka", "", ConfOK},
		{"- 400 g makaronu spaghetti", "makaronu spaghetti", f(400), "g", "", ConfOK},
		{"• 100 ml śmietanki 30%", "śmietanki 30%", f(100), "ml", "", ConfOK},
		{"* 1 l bulionu", "bulionu", f(1), "l", "", ConfOK},
		{"5 dag sera żółtego", "sera żółtego", f(50), "g", "", ConfOK},
		{"2 ząbki czosnku", "czosnku", f(2), "ząbek", "", ConfOK},
		{"3 plasterki boczku", "boczku", f(3), "plaster", "", ConfOK},
		{"1 kostka masła", "masła", f(1), "kostka", "", ConfOK},
		{"1 główka kapusty", "kapusty", f(1), "główka", "", ConfOK},
		{"2 kromki chleba", "chleba", f(2), "kromka", "", ConfOK},
		{"1 pęczek pietruszki", "pietruszki", f(1), "pęczek", "", ConfOK},
		{"Mąka pszenna - 250 g", "Mąka pszenna", f(250), "g", "", ConfGuess},
		{"Cukier – 100 g", "Cukier", f(100), "g", "", ConfGuess},
		{"Jajka: 3", "Jajka", f(3), "szt", "", ConfGuess},
		{"12", "", nil, "", "", ConfUnparsed},
		{"200 g", "", nil, "", "", ConfUnparsed},
		{"", "", nil, "", "", ConfUnparsed},
		{"   ", "", nil, "", "", ConfUnparsed},
		{"Wymieszaj wszystko dokładnie i odstaw na godzinę do lodówki, aż ciasto wyrośnie.", "", nil, "", "", ConfUnparsed},
		{"czosnek 2 ząbki", "czosnek", f(2), "ząbek", "", ConfGuess},
		{"sól 1 łyżeczka", "sól", f(1), "łyżeczka", "", ConfGuess},
		{"sól 1 płaska łyżeczka", "sól", f(1), "łyżeczka", "", ConfGuess},
		{"mąka 500 g", "mąka", f(500), "g", "", ConfGuess},
		{"dynia 1 kg", "dynia", f(1), "kg", "", ConfGuess},
		{"1 płaska łyżeczka cukru", "cukru", f(1), "łyżeczka", "", ConfOK},
		{"2 czubate łyżki mąki", "mąki", f(2), "łyżka", "", ConfOK},
		{"1 duży ząbek czosnku", "czosnku", f(1), "ząbek", "", ConfOK},
		{"śmietanka 30%", "śmietanka 30%", nil, "", "", ConfGuess},
		{"mąka typ 500", "mąka typ 500", nil, "", "", ConfGuess},
		{"sos 1000 wysp", "sos 1000 wysp", nil, "", "", ConfGuess},
		{"???", "", nil, "", "", ConfUnparsed},
		{"...", "", nil, "", "", ConfUnparsed},
		{"0 g mąki", "", nil, "", "", ConfUnparsed},
	}
	for _, c := range cases {
		got := ParseIngredient(c.raw)
		if got.Raw != c.raw {
			t.Errorf("%q: raw=%q", c.raw, got.Raw)
		}
		if got.Name != c.name || got.Unit != c.unit || got.Note != c.note || got.Confidence != c.conf {
			t.Errorf("%q: got name=%q unit=%q note=%q conf=%q; want name=%q unit=%q note=%q conf=%q",
				c.raw, got.Name, got.Unit, got.Note, got.Confidence, c.name, c.unit, c.note, c.conf)
		}
		switch {
		case (got.Quantity == nil) != (c.qty == nil):
			t.Errorf("%q: quantity=%v want %v", c.raw, got.Quantity, c.qty)
		case got.Quantity != nil && (*got.Quantity-*c.qty > 1e-9 || *c.qty-*got.Quantity > 1e-9):
			t.Errorf("%q: quantity=%v want %v", c.raw, *got.Quantity, *c.qty)
		}
	}
}

func TestParseIngredientNeverLosesTheLine(t *testing.T) {
	for _, raw := range []string{"asdf ;;; ((", "1 (", "/", "½", "100000000 g mąki", "1 kg mąki"} {
		got := ParseIngredient(raw)
		if got.Raw != raw {
			t.Errorf("%q: raw lost: %q", raw, got.Raw)
		}
	}
	if got := ParseIngredient("1 kg mąki"); got.Name != "mąki" || got.Unit != "kg" {
		t.Errorf("non-breaking spaces: %+v", got)
	}
}
