// Command api serves the HTTP API. It migrates Postgres and opens the existing
// OpenFGA store; it never seeds.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/google/uuid"

	"github.com/vuhn/fga-demo/internal/adapter/auth"
	httpapi "github.com/vuhn/fga-demo/internal/adapter/http"
	"github.com/vuhn/fga-demo/internal/adapter/openfga"
	"github.com/vuhn/fga-demo/internal/adapter/postgres"
	"github.com/vuhn/fga-demo/internal/platform/config"
	"github.com/vuhn/fga-demo/internal/usecase"
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

	fga, err := openfga.Open(ctx, cfg.OpenFGAAPIURL, cfg.OpenFGAStoreName)
	if err != nil {
		return err
	}
	log.Info("openfga", "store_id", fga.StoreID, "model_id", fga.ModelID)

	tokens := auth.JWT{Secret: []byte(cfg.JWTSecret), TTL: cfg.JWTTTL}
	app := &usecase.App{
		Store:  postgres.NewStore(pool),
		Authz:  fga,
		Tuples: openfga.Catalog{},
		Hasher: auth.Bcrypt{},
		Tokens: tokens,
		NewID:  func() string { return uuid.NewString() },
		Log:    log,
	}
	router := httpapi.NewRouter(app, tokens, cfg.CORSOrigin, log)
	log.Info("listening", "addr", cfg.HTTPAddr)
	return http.ListenAndServe(cfg.HTTPAddr, router)
}
