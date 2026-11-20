package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type blueprintShareRevoker struct {
	blueprints gateway.BlueprintGateway
	grants     gateway.BlueprintGrantGateway
	relations  gateway.BlueprintRelationGateway
	authz      helper.Authorizer
	policy     helper.SharePolicy
	guard      helper.WriteGuard
}

func NewBlueprintShareRevoker(blueprints gateway.BlueprintGateway, grants gateway.BlueprintGrantGateway, relations gateway.BlueprintRelationGateway,
	authz helper.Authorizer, policy helper.SharePolicy, guard helper.WriteGuard) usecase.BlueprintShareRevoker {
	return &blueprintShareRevoker{blueprints: blueprints, grants: grants, relations: relations, authz: authz, policy: policy, guard: guard}
}

// Revoke deletes the matching grant's tuple, then its row.
func (s *blueprintShareRevoker) Revoke(ctx context.Context, in input.BlueprintShareRevoker) error {
	g, err := s.policy.NewGrant(in.TenantID, model.ObjectTypeBlueprint, in.ID, model.ShareInput{Relation: in.Relation, Subject: in.Subject, UserID: in.UserID})
	if err != nil {
		return err
	}
	if err := requireBlueprintShare(ctx, s.blueprints, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
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
