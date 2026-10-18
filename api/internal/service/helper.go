package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

// ---- Write ordering ----

// writeGuard is the only place that orders Postgres and OpenFGA writes.
type writeGuard struct {
	authz gateway.AuthzGateway
	log   *slog.Logger
}

func NewWriteGuard(authz gateway.AuthzGateway, log *slog.Logger) helper.WriteGuard {
	return &writeGuard{authz: authz, log: log}
}

// Grant persists access: Postgres has already committed; now write tuples.
// When OpenFGA fails, undo the Postgres rows and report tuple_write_failed.
func (w *writeGuard) Grant(ctx context.Context, writes, deletes []entity.Tuple, undo func(context.Context) error) error {
	err := w.authz.Write(ctx, writes, deletes)
	if err == nil {
		return nil
	}
	w.log.Error("tuple write failed; compensating postgres", "err", err)
	if cerr := undo(context.WithoutCancel(ctx)); cerr != nil {
		w.log.Error("compensation failed; postgres and openfga may have drifted (fail-closed)", "err", cerr, "tuple_err", err)
	}
	return fmt.Errorf("%w: %v", entity.ErrTupleWriteFailed, err)
}

// Revoke removes access: delete tuples first, then Postgres rows. When
// Postgres fails, restore exactly the tuples that existed before.
func (w *writeGuard) Revoke(ctx context.Context, deletes, restore []entity.Tuple, persist func(context.Context) error) error {
	if err := w.authz.Write(ctx, nil, deletes); err != nil {
		return fmt.Errorf("%w: %v", entity.ErrTupleWriteFailed, err)
	}
	err := persist(ctx)
	if err == nil {
		return nil
	}
	w.log.Error("postgres failed after tuple delete; restoring tuples", "err", err)
	if cerr := w.authz.Write(context.WithoutCancel(ctx), restore, nil); cerr != nil {
		w.log.Error("tuple restore failed; display rows remain without tuples (fail-closed)", "err", cerr, "pg_err", err)
	}
	if errors.Is(err, entity.ErrNotFound) || errors.Is(err, entity.ErrConflict) {
		return err
	}
	return fmt.Errorf("revoke: %w", err)
}

// RevokeObject removes every tuple on the object, then runs persist.
func (w *writeGuard) RevokeObject(ctx context.Context, objectType, objectID string, persist func(context.Context) error) error {
	tuples, err := w.authz.ReadTuples(ctx, objectType, objectID)
	if err != nil {
		return err
	}
	return w.Revoke(ctx, tuples, tuples, persist)
}

// ---- Share rules ----

type sharePolicy struct {
	authz usecase.Authorizer
}

func NewSharePolicy(authz usecase.Authorizer) helper.SharePolicy {
	return &sharePolicy{authz: authz}
}

func (sharePolicy) NewGrant(tenantID, objectType, objectID string, in entity.ShareInput) (entity.Grant, error) {
	switch in.Relation {
	case entity.RelViewer, entity.RelEditor:
	case entity.RelOwner:
		return entity.Grant{}, entity.Invalid("owner cannot be granted or revoked")
	default:
		return entity.Grant{}, entity.Invalid("relation must be viewer or editor")
	}
	g := entity.Grant{TenantID: tenantID, ObjectType: objectType, ObjectID: objectID, Relation: in.Relation, SubjectType: in.Subject}
	if in.Subject == entity.SubjectUser {
		if err := entity.RequireUUID("user_id", in.UserID); err != nil {
			return entity.Grant{}, err
		}
		uid := in.UserID
		g.UserID = &uid
	} else if in.UserID != "" {
		return entity.Grant{}, entity.Invalid("user_id is only allowed with subject user")
	}
	return g, nil
}

func (p *sharePolicy) CheckGrantee(ctx context.Context, tenantID string, g entity.Grant) error {
	if g.UserID == nil {
		return nil
	}
	ok, err := p.authz.Check(ctx, *g.UserID, "can_read_tenant", entity.ObjectTypeTenant, tenantID)
	if err != nil {
		return err
	}
	if !ok {
		return entity.ErrNotAMember
	}
	if g.Relation != entity.RelEditor {
		return nil
	}
	ok, err = p.authz.Check(ctx, *g.UserID, "can_contribute", entity.ObjectTypeTenant, tenantID)
	if err != nil {
		return err
	}
	if !ok {
		return entity.ErrCannotUpload
	}
	return nil
}

// ---- Shared functions ----

// withPerms pairs each object with the actor's permissions on it, so the UI
// needs no extra Check round trips.
func withPerms[T any](ctx context.Context, authz usecase.Authorizer, actor, objectType string, xs []T, id func(T) string) ([]entity.Visible[T], error) {
	out := make([]entity.Visible[T], 0, len(xs))
	if len(xs) == 0 {
		return out, nil
	}
	ids := make([]string, len(xs))
	for i, x := range xs {
		ids[i] = id(x)
	}
	perms, err := authz.Permissions(ctx, actor, objectType, ids)
	if err != nil {
		return nil, err
	}
	for _, x := range xs {
		out = append(out, entity.Visible[T]{Value: x, Perms: perms[id(x)]})
	}
	return out, nil
}

// maxDepth bounds folder and album nesting.
const maxDepth = 10
