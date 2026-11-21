//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/postgres"
	. "github.com/vuhn/fga-demo/api/internal/seed"
)

// Needs the seeded database (DATABASE_URL after `make seed`); removes its rows.
func TestTaskRepoCreateGet(t *testing.T) {
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
	r := postgres.NewTaskRepo(pool)
	const id1, id2, id3 = "e0e0e0e0-0000-4000-8000-000000000001", "e0e0e0e0-0000-4000-8000-000000000002", "e0e0e0e0-0000-4000-8000-000000000003"
	defer pool.Exec(ctx, `DELETE FROM tasks WHERE id = ANY($1)`, []string{id1, id2, id3})

	d := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	full := model.Task{ID: id1, TenantID: Acme, ProjectID: ProjAcmeTower, Title: "Pour slab", Description: "level 2", AssigneeID: ptr(Cara),
		PlannedStart: &d, PlannedEnd: &d, Status: model.TaskInProgress, Weight: 2.25, CreatedBy: Bob}
	if err := r.Create(ctx, full); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, Acme, id1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Pour slab" || got.Weight != 2.25 || got.AssigneeName == nil || *got.AssigneeName != "Cara" || got.PlannedStart == nil ||
		got.PlannedStart.Format("2006-01-02") != "2026-01-05" || got.ActualStart != nil || got.CommentCount != 0 || got.CreatedAt.IsZero() {
		t.Fatalf("round trip: %+v", got)
	}

	// Defaults and no assignee: the name is nil.
	if err := r.Create(ctx, model.Task{ID: id2, TenantID: Acme, ProjectID: ProjAcmeTower, Title: "x", Status: model.TaskNotStarted, Weight: 1, CreatedBy: Bob}); err != nil {
		t.Fatal(err)
	}
	if g, _ := r.Get(ctx, Acme, id2); g.AssigneeName != nil || g.AssigneeID != nil {
		t.Fatalf("unassigned: %+v", g)
	}

	// Another tenant never sees it.
	if _, err := r.Get(ctx, Globex, id1); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("cross-tenant get: %v", err)
	}
	// A project of another tenant violates the composite FK; a CHECK failure is a validation error.
	if err := r.Create(ctx, model.Task{ID: id3, TenantID: Globex, ProjectID: ProjAcmeTower, Title: "x", Status: model.TaskNotStarted, Weight: 1, CreatedBy: Bob}); err == nil {
		t.Fatal("cross-tenant project must fail")
	}
	bad := model.Task{ID: id3, TenantID: Acme, ProjectID: ProjAcmeTower, Title: "x", Status: "done", Weight: 1, CreatedBy: Bob}
	if err := r.Create(ctx, bad); !errors.Is(err, model.ErrValidation) {
		t.Fatalf("bad status: %v", err)
	}
	bad.Status, bad.Weight = model.TaskNotStarted, 0
	if err := r.Create(ctx, bad); !errors.Is(err, model.ErrValidation) {
		t.Fatalf("weight 0: %v", err)
	}
	bad.Weight, bad.PlannedStart, bad.PlannedEnd = 1, &d, ptr2(d.AddDate(0, 0, -1))
	if err := r.Create(ctx, bad); !errors.Is(err, model.ErrValidation) {
		t.Fatalf("end before start: %v", err)
	}
}

func ptr(s string) *string        { return &s }
func ptr2(t time.Time) *time.Time { return &t }
