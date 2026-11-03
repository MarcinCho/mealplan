package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mealplan/internal/auth"
	"mealplan/internal/config"
	"mealplan/internal/httpapi"
	"mealplan/internal/importer"
	"mealplan/internal/plan"
	"mealplan/internal/recipe"
	"mealplan/internal/shopping"
	"mealplan/internal/spa"
	"mealplan/internal/store"
	"mealplan/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	db, err := store.Open(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := db.Migrate(ctx); err != nil {
		return err
	}
	st := store.New(db)
	svc, err := auth.NewService(st, auth.Config{
		SessionIdle: cfg.SessionIdle,
		SessionMax:  cfg.SessionMax,
		InviteTTL:   72 * time.Hour,
		Hasher:      auth.DefaultHasher,
	}, nil)
	if err != nil {
		return err
	}
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(svc, httpapi.Deps{Recipes: recipe.New(st, nil), Plan: plan.New(st, st), Shopping: shopping.New(st, st), Importer: importer.New()}, cfg, spa.Handler(dist), log).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "addr", cfg.Addr, "mode", cfg.Mode, "db", cfg.DBDriver)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
