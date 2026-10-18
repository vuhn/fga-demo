// Command seed resets the demo: it migrates and truncates Postgres, recreates
// the OpenFGA store, and writes the fixed world from ARCHITECTURE.md.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"

	"github.com/vuhn/fga-demo/internal/adapter/auth"
	"github.com/vuhn/fga-demo/internal/adapter/openfga"
	"github.com/vuhn/fga-demo/internal/adapter/postgres"
	"github.com/vuhn/fga-demo/internal/domain"
	"github.com/vuhn/fga-demo/internal/platform/config"
	"github.com/vuhn/fga-demo/internal/seed"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
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

	fga, err := openfga.Reset(ctx, cfg.OpenFGAAPIURL, cfg.OpenFGAStoreName, cfg.OpenFGAModelPath)
	if err != nil {
		return err
	}
	fmt.Printf("store_id=%s\nmodel_id=%s\n", fga.StoreID, fga.ModelID)

	if err := postgres.Truncate(ctx, pool); err != nil {
		return err
	}
	store := postgres.NewStore(pool)
	hash, err := auth.Bcrypt{}.Hash("password")
	if err != nil {
		return err
	}
	for _, p := range seed.People {
		if err := store.CreateUser(ctx, domain.User{ID: p.ID, Email: p.Email, Name: p.Name, PasswordHash: hash, IsSystemAdmin: p.SystemAdmin}); err != nil {
			return fmt.Errorf("user %s: %w", p.Name, err)
		}
	}
	for _, t := range seed.Tenants {
		if err := store.CreateTenant(ctx, domain.Tenant{ID: t.ID, Slug: t.Slug, Name: t.Name}); err != nil {
			return fmt.Errorf("tenant %s: %w", t.Name, err)
		}
		for _, prod := range []string{domain.ProductDocuments, domain.ProductPhotos} {
			if _, err := store.SetProduct(ctx, t.ID, prod, true); err != nil {
				return err
			}
		}
	}
	for _, m := range seed.Members {
		if err := store.InsertMembership(ctx, m); err != nil {
			return fmt.Errorf("membership: %w", err)
		}
	}

	// Insert each object with its owner grant, then the remaining grants.
	grants := seed.Grants()
	owners := map[string]domain.Grant{}
	var shares []domain.Grant
	for _, g := range grants {
		g.ID = uuid.NewString()
		if g.Relation == domain.RelOwner {
			owners[g.ObjectID] = g
		} else {
			shares = append(shares, g)
		}
	}
	for _, c := range seed.Containers {
		err := store.CreateContainer(ctx, c.Kind, domain.Container{ID: c.ID, TenantID: c.Tenant, ParentID: c.Parent, Name: c.Name, CreatedBy: c.Owner, IsPrivate: c.Private}, owners[c.ID])
		if err != nil {
			return fmt.Errorf("%s %s: %w", c.Kind.Container, c.Name, err)
		}
	}
	for _, it := range seed.Items {
		mime, size := "text/plain", int64(128)
		if it.Kind == domain.Photos {
			mime, size = "image/jpeg", 2048
		}
		err := store.CreateItem(ctx, it.Kind, domain.Item{ID: it.ID, TenantID: it.Tenant, ContainerID: it.Container, Name: it.Name, MimeType: mime, ByteSize: size, CreatedBy: it.Owner}, owners[it.ID])
		if err != nil {
			return fmt.Errorf("%s %s: %w", it.Kind.Item, it.Name, err)
		}
	}
	for _, g := range shares {
		if _, err := store.UpsertGrant(ctx, g); err != nil {
			return fmt.Errorf("grant: %w", err)
		}
	}

	tuples, err := seed.Tuples()
	if err != nil {
		return err
	}
	// Seeding is not a domain command; chunking is fine here.
	for i := 0; i < len(tuples); i += 100 {
		if err := fga.Write(ctx, tuples[i:min(i+100, len(tuples))], nil); err != nil {
			return err
		}
	}
	fmt.Printf("seeded %d users, %d tenants, %d tuples\n", len(seed.People), len(seed.Tenants), len(tuples))
	return nil
}
