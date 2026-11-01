package main

import (
	"log"
	"net/http"
	"os"

	"mealplan/internal/config"
)

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	log.Printf("mealplan (%s) listening on %s", cfg.Mode, cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, mux))
}
