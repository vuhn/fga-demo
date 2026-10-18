package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// authorizer answers every allow/deny with OpenFGA through the authz gateway.
type authorizer struct {
	authz gateway.AuthzGateway
}

func NewAuthorizer(authz gateway.AuthzGateway) usecase.Authorizer {
	return &authorizer{authz: authz}
}

func (a *authorizer) Check(ctx context.Context, actor, relation, objectType, objectID string) (bool, error) {
	return a.authz.Check(ctx, actor, relation, objectType, objectID)
}

func (a *authorizer) Require(ctx context.Context, actor, relation, objectType, objectID string) error {
	return a.require(ctx, entity.ErrForbidden, actor, relation, objectType, objectID)
}

func (a *authorizer) RequireVisible(ctx context.Context, actor, relation, objectType, objectID string) error {
	return a.require(ctx, entity.ErrNotFound, actor, relation, objectType, objectID)
}

func (a *authorizer) require(ctx context.Context, failErr error, actor, relation, objectType, objectID string) error {
	ok, err := a.Check(ctx, actor, relation, objectType, objectID)
	if err != nil {
		return err
	}
	if !ok {
		return entity.Denied(failErr, relation, objectType, objectID)
	}
	return nil
}

func (a *authorizer) RequirePlatform(ctx context.Context, actor string) error {
	return a.Require(ctx, actor, "can_manage_platform", entity.ObjectTypePlatform, entity.PlatformID)
}

func (a *authorizer) IsPlatformAdmin(ctx context.Context, actor string) (bool, error) {
	return a.Check(ctx, actor, "can_manage_platform", entity.ObjectTypePlatform, entity.PlatformID)
}

func (a *authorizer) PlatformOr(ctx context.Context, actor, tenantRelation, tenantID string, failErr error) error {
	ok, err := a.IsPlatformAdmin(ctx, actor)
	if err != nil || ok {
		return err
	}
	return a.require(ctx, failErr, actor, tenantRelation, entity.ObjectTypeTenant, tenantID)
}

func (a *authorizer) ListObjectIDs(ctx context.Context, actor, relation, objectType string) ([]string, error) {
	return a.authz.ListObjectIDs(ctx, actor, relation, objectType)
}

func (a *authorizer) Permissions(ctx context.Context, actor, objectType string, ids []string) (map[string]entity.Perms, error) {
	rels := entity.PermissionRelations[objectType]
	checks := make([]entity.CheckRequest, 0, len(ids)*len(rels))
	for _, id := range ids {
		for _, r := range rels {
			checks = append(checks, entity.CheckRequest{UserID: actor, Relation: r, ObjectType: objectType, ObjectID: id})
		}
	}
	res, err := a.authz.BatchCheck(ctx, checks)
	if err != nil {
		return nil, err
	}
	out := make(map[string]entity.Perms, len(ids))
	for i, c := range checks {
		if out[c.ObjectID] == nil {
			out[c.ObjectID] = entity.Perms{}
		}
		out[c.ObjectID][c.Relation] = res[i]
	}
	return out, nil
}

func (a *authorizer) BatchCheck(ctx context.Context, checks []entity.CheckRequest) ([]bool, error) {
	return a.authz.BatchCheck(ctx, checks)
}
