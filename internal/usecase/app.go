package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/vuhn/fga-demo/internal/domain"
)

// App bundles the ports. Methods are grouped by area across this package.
type App struct {
	Store  Store
	Authz  Authorizer
	Tuples TupleCatalog
	Hasher PasswordHasher
	Tokens TokenIssuer
	NewID  IDGenerator
	Log    *slog.Logger
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ValidUUID(s string) bool { return uuidRe.MatchString(s) }

func requireUUID(field, s string) error {
	if !ValidUUID(s) {
		return domain.Invalid("%s must be a UUID", field)
	}
	return nil
}

func cleanName(field, s string) (string, error) {
	s = strings.TrimSpace(s)
	if n := len([]rune(s)); n < 1 || n > 200 {
		return "", domain.Invalid("%s length must be 1 to 200", field)
	}
	return s, nil
}

// ---- Checks ----

func (a *App) check(ctx context.Context, actor, relation, objectType, objectID string) (bool, error) {
	return a.Authz.Check(ctx, actor, relation, objectType, objectID)
}

func denied(err error, relation, objectType, objectID string) error {
	return &domain.CheckError{Err: err, Relation: relation, Object: objectType + ":" + objectID}
}

// require returns 403 when the Check is false. Writes use it.
func (a *App) require(ctx context.Context, actor, relation, objectType, objectID string) error {
	ok, err := a.check(ctx, actor, relation, objectType, objectID)
	if err != nil {
		return err
	}
	if !ok {
		return denied(domain.ErrForbidden, relation, objectType, objectID)
	}
	return nil
}

// requireVisible returns 404 when the Check is false, so GET-by-id does not
// reveal existence.
func (a *App) requireVisible(ctx context.Context, actor, relation, objectType, objectID string) error {
	ok, err := a.check(ctx, actor, relation, objectType, objectID)
	if err != nil {
		return err
	}
	if !ok {
		return denied(domain.ErrNotFound, relation, objectType, objectID)
	}
	return nil
}

// platformOr allows the action when the actor can manage the platform OR holds
// tenantRelation on the tenant. Two Checks composed here, never a role test.
// The returned error unwraps to failErr.
func (a *App) platformOr(ctx context.Context, actor, tenantRelation, tenantID string, failErr error) error {
	ok, err := a.check(ctx, actor, "can_manage_platform", "platform", domain.PlatformID)
	if err != nil || ok {
		return err
	}
	ok, err = a.check(ctx, actor, tenantRelation, "tenant", tenantID)
	if err != nil {
		return err
	}
	if !ok {
		return denied(failErr, tenantRelation, "tenant", tenantID)
	}
	return nil
}

// ---- Write path ----

// grant persists access: Postgres has already committed; now write tuples.
// When OpenFGA fails, undo the Postgres rows and report tuple_write_failed.
func (a *App) grant(ctx context.Context, writes, deletes []Tuple, undo func(context.Context) error) error {
	err := a.Authz.Write(ctx, writes, deletes)
	if err == nil {
		return nil
	}
	a.Log.Error("tuple write failed; compensating postgres", "err", err)
	if cerr := undo(context.WithoutCancel(ctx)); cerr != nil {
		a.Log.Error("compensation failed; postgres and openfga may have drifted (fail-closed)", "err", cerr, "tuple_err", err)
	}
	return fmt.Errorf("%w: %v", domain.ErrTupleWriteFailed, err)
}

// revoke removes access: delete tuples first, then Postgres rows. When
// Postgres fails, restore exactly the tuples that existed before.
func (a *App) revoke(ctx context.Context, deletes, restore []Tuple, persist func(context.Context) error) error {
	if err := a.Authz.Write(ctx, nil, deletes); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrTupleWriteFailed, err)
	}
	err := persist(ctx)
	if err == nil {
		return nil
	}
	a.Log.Error("postgres failed after tuple delete; restoring tuples", "err", err)
	if cerr := a.Authz.Write(context.WithoutCancel(ctx), restore, nil); cerr != nil {
		a.Log.Error("tuple restore failed; display rows remain without tuples (fail-closed)", "err", cerr, "pg_err", err)
	}
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrConflict) {
		return err
	}
	return fmt.Errorf("revoke: %w", err)
}
