package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type photoShareRevoker struct {
	photos    gateway.PhotoGateway
	grants    gateway.PhotoGrantGateway
	relations gateway.PhotoRelationGateway
	authz     helper.Authorizer
	policy    helper.SharePolicy
	guard     helper.WriteGuard
}

func NewPhotoShareRevoker(photos gateway.PhotoGateway, grants gateway.PhotoGrantGateway, relations gateway.PhotoRelationGateway,
	authz helper.Authorizer, policy helper.SharePolicy, guard helper.WriteGuard) usecase.PhotoShareRevoker {
	return &photoShareRevoker{photos: photos, grants: grants, relations: relations, authz: authz, policy: policy, guard: guard}
}

// Revoke deletes the matching grant's tuple, then its row.
func (s *photoShareRevoker) Revoke(ctx context.Context, in input.PhotoShareRevoker) error {
	g, err := s.policy.NewGrant(in.TenantID, model.ObjectTypePhoto, in.ID, model.ShareInput{Relation: in.Relation, Subject: in.Subject, UserID: in.UserID})
	if err != nil {
		return err
	}
	if err := requirePhotoShare(ctx, s.photos, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return err
	}
	existing, err := s.grants.List(ctx, in.ID)
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
		return s.guard.Revoke(ctx, []model.Tuple{t}, []model.Tuple{t}, func(ctx context.Context) error {
			return s.grants.Delete(ctx, e.ID)
		})
	}
	return model.ErrNotFound
}
