// Command seed resets the demo: it migrates and truncates Postgres, recreates
// the OpenFGA store, and writes the fixed world from docs/seed-data.md.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/auth"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/config"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/fga"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/postgres"
	"github.com/vuhn/fga-demo/api/internal/seed"
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

	authz, err := fga.Reset(ctx, cfg.OpenFGAAPIURL, cfg.OpenFGAStoreName, cfg.OpenFGAModelPath)
	if err != nil {
		return err
	}
	fmt.Printf("store_id=%s\nmodel_id=%s\n", authz.StoreID, authz.ModelID)

	if err := postgres.Truncate(ctx, pool); err != nil {
		return err
	}
	var (
		users     = postgres.NewUserRepo(pool)
		tenants   = postgres.NewTenantRepo(pool)
		products  = postgres.NewProductRepo(pool)
		members   = postgres.NewMembershipRepo(pool)
		roles     = postgres.NewRoleRepo(pool)
		folders   = postgres.NewFolderRepo(pool)
		documents = postgres.NewDocumentRepo(pool)
		albums    = postgres.NewAlbumRepo(pool)
		photos    = postgres.NewPhotoRepo(pool)

		folderGrants = postgres.NewFolderGrantRepo(pool)
		albumGrants  = postgres.NewAlbumGrantRepo(pool)
	)
	hash, err := auth.Bcrypt{}.Hash("password")
	if err != nil {
		return err
	}
	for _, p := range seed.People {
		if err := users.Create(ctx, entity.User{ID: p.ID, Email: p.Email, Name: p.Name, PasswordHash: hash, IsSystemAdmin: p.SystemAdmin}); err != nil {
			return fmt.Errorf("user %s: %w", p.Name, err)
		}
	}
	for _, t := range seed.Tenants {
		if err := tenants.Create(ctx, entity.Tenant{ID: t.ID, Slug: t.Slug, Name: t.Name}); err != nil {
			return fmt.Errorf("tenant %s: %w", t.Name, err)
		}
		for _, prod := range entity.AllProducts {
			if _, err := products.Set(ctx, t.ID, prod, true); err != nil {
				return err
			}
		}
	}
	for _, m := range seed.Members {
		if err := members.Insert(ctx, m); err != nil {
			return fmt.Errorf("membership: %w", err)
		}
	}
	for _, role := range seed.Roles {
		if err := roles.Insert(ctx, role); err != nil {
			return fmt.Errorf("role %s: %w", role.Name, err)
		}
		if err := roles.InsertPermissions(ctx, role.ID, role.Permissions); err != nil {
			return fmt.Errorf("role %s permissions: %w", role.Name, err)
		}
	}

	// Insert each object with its owner grant, then the remaining grants.
	owners := map[string]entity.Grant{}
	var shares []entity.Grant
	for _, g := range seed.Grants() {
		g.ID = uuid.NewString()
		if g.Relation == entity.RelOwner {
			owners[g.ObjectID] = g
		} else {
			shares = append(shares, g)
		}
	}
	for _, s := range seed.Folders {
		if err := folders.Create(ctx, s.Folder, owners[s.Folder.ID]); err != nil {
			return fmt.Errorf("folder %s: %w", s.Folder.Name, err)
		}
	}
	for _, s := range seed.Albums {
		if err := albums.Create(ctx, s.Album, owners[s.Album.ID]); err != nil {
			return fmt.Errorf("album %s: %w", s.Album.Name, err)
		}
	}
	for _, d := range seed.Documents {
		if err := documents.Create(ctx, d, owners[d.ID]); err != nil {
			return fmt.Errorf("document %s: %w", d.Name, err)
		}
	}
	for _, p := range seed.Photos {
		if err := photos.Create(ctx, p, owners[p.ID]); err != nil {
			return fmt.Errorf("photo %s: %w", p.Name, err)
		}
	}
	// Only folders and albums carry seeded shares.
	for _, g := range shares {
		var err error
		switch g.ObjectType {
		case entity.ObjectTypeFolder:
			_, err = folderGrants.Upsert(ctx, g)
		case entity.ObjectTypeAlbum:
			_, err = albumGrants.Upsert(ctx, g)
		default:
			err = fmt.Errorf("no seeded shares on %s", g.ObjectType)
		}
		if err != nil {
			return fmt.Errorf("grant: %w", err)
		}
	}

	tuples, err := seed.Tuples()
	if err != nil {
		return err
	}
	// Seeding is not a domain command; chunking is fine here.
	for i := 0; i < len(tuples); i += 100 {
		if err := authz.Write(ctx, tuples[i:min(i+100, len(tuples))], nil); err != nil {
			return err
		}
	}
	fmt.Printf("seeded %d users, %d tenants, %d tuples\n", len(seed.People), len(seed.Tenants), len(tuples))
	return nil
}
