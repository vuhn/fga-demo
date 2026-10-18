package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type photoShareRevoker struct {
	photos    gateway.PhotoGateway
	grants    gateway.PhotoGrantGateway
	relations gateway.PhotoRelationGateway
	authz     usecase.Authorizer
	policy    helper.SharePolicy
	guard     helper.WriteGuard
}

func NewPhotoShareRevoker(photos gateway.PhotoGateway, grants gateway.PhotoGrantGateway, relations gateway.PhotoRelationGateway,
	authz usecase.Authorizer, policy helper.SharePolicy, guard helper.WriteGuard) usecase.PhotoShareRevoker {
	return &photoShareRevoker{photos: photos, grants: grants, relations: relations, authz: authz, policy: policy, guard: guard}
}

// Revoke deletes the matching grant's tuple, then its row.
func (s *photoShareRevoker) Revoke(ctx context.Context, actor, tenantID, id string, in entity.ShareInput) error {
	g, err := s.policy.NewGrant(tenantID, entity.ObjectTypePhoto, id, in)
	if err != nil {
		return err
	}
	if err := requirePhotoShare(ctx, s.photos, s.authz, actor, tenantID, id); err != nil {
		return err
	}
	existing, err := s.grants.List(ctx, id)
	if err != nil {
		return err
	}
	for _, e := range existing {
		if !e.SameSubject(g) || e.Relation != g.Relation {
			continue
		}
		t, err := s.relations.Grant(e)
		if err != nil {
			return err
		}
		return s.guard.Revoke(ctx, []entity.Tuple{t}, []entity.Tuple{t}, func(ctx context.Context) error {
			return s.grants.Delete(ctx, e.ID)
		})
	}
	return entity.ErrNotFound
}
