// Command api serves the HTTP API. It migrates Postgres and opens the existing
// OpenFGA store; it never seeds.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/vuhn/fga-demo/api/internal/infrastructure/config"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/fga"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/postgres"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if err := run(log); err != nil {
		log.Error("api stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	authz, err := fga.Open(ctx, cfg.OpenFGAAPIURL, cfg.OpenFGAStoreName)
	if err != nil {
		return err
	}
	log.Info("openfga", "store_id", authz.StoreID, "model_id", authz.ModelID)

	router := wire(cfg, pool, authz, log)
	log.Info("listening", "addr", cfg.HTTPAddr)
	return http.ListenAndServe(cfg.HTTPAddr, router)
}
