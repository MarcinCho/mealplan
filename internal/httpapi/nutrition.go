package httpapi

import (
	"net/http"

	"mealplan/internal/domain"
	"mealplan/internal/nutrition"
)

type settingsJSON struct {
	Enabled bool       `json:"enabled"`
	Goal    macrosJSON `json:"goal"`
}

type mealJSON struct {
	EntryID     string     `json:"entryId"`
	Title       string     `json:"title"`
	Slot        string     `json:"slot"`
	Values      macrosJSON `json:"values"`
	Sources     sourcesOut `json:"sources"`
	Approximate bool       `json:"approximate"`
	Missing     bool       `json:"missing"`
}

type sourcesOut struct {
	Kcal    string `json:"kcal"`
	Protein string `json:"protein"`
	Carbs   string `json:"carbs"`
	Fat     string `json:"fat"`
}

type logJSON struct {
	ID        string     `json:"id"`
	ProductID string     `json:"productId"`
	Name      string     `json:"name"`
	Quantity  float64    `json:"quantity"`
	Unit      string     `json:"unit"`
	Values    macrosJSON `json:"values"`
	Missing   bool       `json:"missing"`
}

type incompleteJSON struct {
	Kcal    bool `json:"kcal"`
	Protein bool `json:"protein"`
	Carbs   bool `json:"carbs"`
	Fat     bool `json:"fat"`
}

type dayJSON struct {
	Date         string         `json:"date"`
	Total        macrosJSON     `json:"total"`
	Goal         macrosJSON     `json:"goal"`
	Remaining    macrosJSON     `json:"remaining"`
	Meals        []mealJSON     `json:"meals"`
	Log          []logJSON      `json:"log"`
	MissingMeals int            `json:"missingMeals"`
	MissingLog   int            `json:"missingLog"`
	Incomplete   incompleteJSON `json:"incomplete"`
	Approximate  bool           `json:"approximate"`
}

func toDayJSON(d nutrition.DaySummary) dayJSON {
	out := dayJSON{Date: d.Date, Total: macrosJSON(d.Total), Goal: macrosJSON(d.Goal), Remaining: macrosJSON(d.Remaining),
		Meals: make([]mealJSON, 0, len(d.Meals)), Log: make([]logJSON, 0, len(d.Log)),
		MissingMeals: d.MissingMeals, MissingLog: d.MissingLog, Incomplete: incompleteJSON(d.Incomplete), Approximate: d.Approximate}
	for _, m := range d.Meals {
		out.Meals = append(out.Meals, mealJSON{EntryID: m.EntryID, Title: m.Title, Slot: string(m.Slot), Values: macrosJSON(m.Values),
			Sources:     sourcesOut{string(m.Sources.Kcal), string(m.Sources.Protein), string(m.Sources.Carbs), string(m.Sources.Fat)},
			Approximate: m.Approximate, Missing: m.Missing})
	}
	for _, l := range d.Log {
		out.Log = append(out.Log, logJSON{ID: l.ID, ProductID: l.ProductID, Name: l.Name, Quantity: l.Quantity, Unit: l.Unit,
			Values: macrosJSON(l.Values), Missing: l.Missing})
	}
	return out
}

func (s *Server) handleGetNutrition(w http.ResponseWriter, r *http.Request) {
	set, err := s.deps.Nutrition.Settings(r.Context(), principalFrom(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, settingsJSON{Enabled: set.Enabled, Goal: macrosJSON(set.Goal)})
}

func (s *Server) handlePutNutrition(w http.ResponseWriter, r *http.Request) {
	var in settingsJSON
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r)
	if err := s.deps.Nutrition.SaveSettings(r.Context(), p, in.Enabled, domain.Macros(in.Goal)); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleNutritionDays(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r)
	set, err := s.deps.Nutrition.Settings(r.Context(), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	days, err := s.deps.Nutrition.Days(r.Context(), p, q.Get("from"), q.Get("to"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]dayJSON, 0, len(days))
	for _, d := range days {
		out = append(out, toDayJSON(d))
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": set.Enabled, "days": out})
}

func (s *Server) handleAddFoodLog(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date      string  `json:"date"`
		ProductID string  `json:"productId"`
		Quantity  float64 `json:"quantity"`
		Unit      string  `json:"unit"`
	}
	if !decode(w, r, &in) {
		return
	}
	e, err := s.deps.Nutrition.AddLog(r.Context(), principalFrom(r), in.Date, in.ProductID, in.Quantity, in.Unit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": e.ID, "date": e.Date, "productId": e.ProductID, "quantity": e.Quantity, "unit": e.Unit})
}

func (s *Server) handleDeleteFoodLog(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Nutrition.DeleteLog(r.Context(), principalFrom(r), r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
