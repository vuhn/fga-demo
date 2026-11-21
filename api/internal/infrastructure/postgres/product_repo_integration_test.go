//go:build integration

package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/postgres"
)

// Needs a migrated database (DATABASE_URL); uses a throwaway tenant row and removes it.
func TestProductRepoSetCreatesMissingRow(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	const tid = "d0d0d0d0-0000-4000-8000-0000000000aa"
	if _, err := pool.Exec(ctx, `INSERT INTO tenants (id, slug, name) VALUES ($1, 'product-repo-test', 'product-repo-test')`, tid); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tid)

	r := postgres.NewProductRepo(pool)
	prev, err := r.Set(ctx, tid, "tasks", true) // no row yet: created disabled, then enabled
	if err != nil || prev {
		t.Fatalf("first Set: prev=%v err=%v", prev, err)
	}
	prev, err = r.Set(ctx, tid, "tasks", true)
	if err != nil || !prev {
		t.Fatalf("second Set: prev=%v err=%v", prev, err)
	}
	if prev, err = r.Set(ctx, tid, "tasks", false); err != nil || !prev {
		t.Fatalf("disable: prev=%v err=%v", prev, err)
	}
	got, err := r.List(ctx, tid)
	if err != nil || len(got) != 1 || got[0].Name != "tasks" || got[0].Enabled {
		t.Fatalf("list: %+v %v", got, err)
	}
}

func TestTenantCreateInsertsFourProductRows(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	const tid = "d0d0d0d0-0000-4000-8000-0000000000bb"
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM companies WHERE tenant_id = $1 OR created_by_tenant_id = $1`, tid)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tid)
	}()
	if err := postgres.NewTenantRepo(pool).Create(ctx, model.Tenant{ID: tid, Slug: "tenant-create-test", Name: "Tenant Create Test"}); err != nil {
		t.Fatal(err)
	}
	got, err := postgres.NewProductRepo(pool).List(ctx, tid)
	if err != nil || len(got) != 4 {
		t.Fatalf("want 4 products, got %+v (%v)", got, err)
	}
	for _, p := range got {
		if p.Enabled || !model.ValidProduct(p.Name) {
			t.Errorf("unexpected product row %+v", p)
		}
	}
}
