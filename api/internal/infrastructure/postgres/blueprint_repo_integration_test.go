//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/postgres"
	. "github.com/vuhn/fga-demo/api/internal/seed"
)

// These tests need the seeded database (DATABASE_URL after `make seed`). Each one creates its own blueprint and
// removes it. They run real transactions against Postgres: the version guard is SQL, so a fake cannot test it.

func newTestBlueprint(t *testing.T, id string) (*pgxpool.Pool, *postgres.BlueprintRepo) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	r := postgres.NewBlueprintRepo(pool)
	b := model.Blueprint{ID: id, TenantID: Acme, ProjectID: ProjAcmeTower, Name: "concurrency " + id[len(id)-4:], SourceKind: "prompt", CreatedBy: Bob}
	v := testVersion("created", "create")
	v.BlueprintID = id
	owner := model.OwnerGrant("e1e1e1e1-0000-4000-8000-0000000000"+id[len(id)-2:], Acme, model.ObjectTypeBlueprint, id, Bob)
	if err := r.Create(ctx, b, v, owner); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		r.Delete(ctx, id)
		pool.Close()
	})
	return pool, r
}

func testVersion(msg, kind string) model.BlueprintVersion {
	return model.BlueprintVersion{BlueprintID: "", Kind: kind, Message: msg, Brief: json.RawMessage(`{"a": 1}`),
		Model: json.RawMessage(`{"m": 1}`), Checks: json.RawMessage(`[]`), OK: true, CreatedBy: Bob}
}

func versionNumbers(t *testing.T, r *postgres.BlueprintRepo, id string) []int {
	t.Helper()
	vs, err := r.ListVersions(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var out []int
	for _, v := range vs {
		out = append(out, v.Version)
	}
	sort.Ints(out)
	return out
}

// Invariants HistContiguous, OneWriterPerVersion, WriteFromBase: many writers on the same base, exactly one wins.
func TestBlueprintAppendVersionConcurrentWritersOneWins(t *testing.T) {
	const id = "e1e1e1e1-0000-4000-8000-000000000101"
	_, r := newTestBlueprint(t, id)
	ctx := context.Background()

	const n = 8
	errs := make([]error, n)
	got := make([]model.BlueprintVersion, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v := testVersion(fmt.Sprintf("writer %d", i), model.BlueprintVersionRevise)
			v.BlueprintID = id
			<-start
			_, got[i], errs[i] = r.AppendVersion(ctx, v, 1)
		}(i)
	}
	close(start)
	wg.Wait()

	winner := -1
	for i, err := range errs {
		switch {
		case err == nil && winner == -1:
			winner = i
		case err == nil:
			t.Fatalf("two writers won on the same base: %d and %d", winner, i)
		case !errors.Is(err, model.ErrConflict):
			t.Fatalf("writer %d: want ErrConflict, got %v", i, err)
		}
	}
	if winner == -1 {
		t.Fatal("no writer won")
	}
	if nums := versionNumbers(t, r, id); len(nums) != 2 || nums[0] != 1 || nums[1] != 2 {
		t.Fatalf("history %v, want [1 2]", nums)
	}
	v2, err := r.GetVersion(ctx, id, 2)
	if err != nil || v2.Message != fmt.Sprintf("writer %d", winner) || got[winner].Version != 2 {
		t.Fatalf("version 2 = %+v (%v), want the winner's row", v2, err)
	}
	if b, _ := r.Get(ctx, id); b.CurrentVersion != 2 {
		t.Fatalf("current_version %d, want 2", b.CurrentVersion)
	}
}

// Same invariants for convert: the original file is stored only for the winner's version.
func TestBlueprintAppendVersionWithOriginalConcurrentWritersOneWins(t *testing.T) {
	const id = "e1e1e1e1-0000-4000-8000-000000000102"
	pool, r := newTestBlueprint(t, id)
	ctx := context.Background()

	const n = 6
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v := testVersion("converted", model.BlueprintVersionEdit)
			v.BlueprintID = id
			o := model.BlueprintOriginal{BlueprintID: id, Version: 2, Name: fmt.Sprintf("w%d.dxf", i), MimeType: model.DXFMimeType, ByteSize: 3, SHA256: "x", Data: []byte("DXF")}
			<-start
			_, _, errs[i] = r.AppendVersionWithOriginal(ctx, v, 1, o)
		}(i)
	}
	close(start)
	wg.Wait()

	wins, winner := 0, -1
	for i, err := range errs {
		if err == nil {
			wins, winner = wins+1, i
		} else if !errors.Is(err, model.ErrConflict) {
			t.Fatalf("want ErrConflict, got %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("%d writers won, want 1", wins)
	}
	var originals int
	var name string
	if err := pool.QueryRow(ctx, `SELECT count(*), coalesce(min(name), '') FROM blueprint_originals WHERE blueprint_id = $1 AND version = 2`, id).Scan(&originals, &name); err != nil || originals != 1 {
		t.Fatalf("originals for version 2: %d (%v), want 1", originals, err)
	}
	if want := fmt.Sprintf("w%d.dxf", winner); name != want {
		t.Fatalf("the stored original is %q, want the winner's %q", name, want)
	}
	if nums := versionNumbers(t, r, id); len(nums) != 2 {
		t.Fatalf("history %v, want [1 2]", nums)
	}
}

// Invariant RespConsistent, and the regression test for the naive counterexample (TLA+ FIXED = FALSE): writers
// that retry on 409 all get a response that describes their own write, while others keep writing around them.
func TestBlueprintAppendVersionResponseIsFromTheWrite(t *testing.T) {
	const id = "e1e1e1e1-0000-4000-8000-000000000103"
	_, r := newTestBlueprint(t, id)
	ctx := context.Background()

	const n = 6
	type result struct {
		b model.Blueprint
		v model.BlueprintVersion
	}
	res := make([]result, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			for {
				cur, err := r.Get(ctx, id)
				if err != nil {
					errs[i] = err
					return
				}
				v := testVersion(fmt.Sprintf("writer %d", i), model.BlueprintVersionRevise)
				v.BlueprintID = id
				b, got, err := r.AppendVersion(ctx, v, cur.CurrentVersion)
				if errors.Is(err, model.ErrConflict) {
					continue // lost the race: reload and write again, like the web page does
				}
				res[i], errs[i] = result{b, got}, err
				return
			}
		}(i)
	}
	close(start)
	wg.Wait()

	seen := map[int]bool{}
	for i := range res {
		if errs[i] != nil {
			t.Fatalf("writer %d: %v", i, errs[i])
		}
		if res[i].b.CurrentVersion != res[i].v.Version {
			t.Fatalf("writer %d: blueprint.current_version %d next to version %d", i, res[i].b.CurrentVersion, res[i].v.Version)
		}
		if res[i].v.Message != fmt.Sprintf("writer %d", i) || res[i].v.BlueprintID != id || res[i].v.CreatedAt.IsZero() {
			t.Fatalf("writer %d got another writer's row: %+v", i, res[i].v)
		}
		if seen[res[i].v.Version] {
			t.Fatalf("version %d written twice", res[i].v.Version)
		}
		seen[res[i].v.Version] = true
	}
	nums := versionNumbers(t, r, id)
	if len(nums) != n+1 || nums[0] != 1 || nums[n] != n+1 {
		t.Fatalf("history %v, want 1..%d", nums, n+1)
	}
}

// The returned rows read like Get and GetVersion read them (same JSON normalization, notes never nil).
func TestBlueprintAppendVersionReturnsTheStoredRows(t *testing.T) {
	const id = "e1e1e1e1-0000-4000-8000-000000000104"
	_, r := newTestBlueprint(t, id)
	ctx := context.Background()
	v := testVersion("edited", model.BlueprintVersionEdit)
	v.BlueprintID, v.Checks, v.Notes = id, nil, nil
	b, got, err := r.AppendVersion(ctx, v, 1)
	if err != nil {
		t.Fatal(err)
	}
	wantB, _ := r.Get(ctx, id)
	wantV, _ := r.GetVersion(ctx, id, 2)
	jb, _ := json.Marshal(b)
	jwb, _ := json.Marshal(wantB)
	jv, _ := json.Marshal(got)
	jwv, _ := json.Marshal(wantV)
	if string(jb) != string(jwb) || string(jv) != string(jwv) {
		t.Fatalf("returned rows differ from the stored ones:\n%s\n%s\n%s\n%s", jb, jwb, jv, jwv)
	}
}

// Accepted behaviour (F5 in the contract): a write that races a delete is a 409, and nothing is stored.
func TestBlueprintAppendVersionAfterDeleteIsConflict(t *testing.T) {
	const id = "e1e1e1e1-0000-4000-8000-000000000105"
	_, r := newTestBlueprint(t, id)
	ctx := context.Background()
	if err := r.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	v := testVersion("late", model.BlueprintVersionRevise)
	v.BlueprintID = id
	if _, _, err := r.AppendVersion(ctx, v, 1); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	if nums := versionNumbers(t, r, id); len(nums) != 0 {
		t.Fatalf("versions left: %v", nums)
	}
}
