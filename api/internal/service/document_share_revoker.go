package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type documentShareRevoker struct {
	documents gateway.DocumentGateway
	grants    gateway.DocumentGrantGateway
	relations gateway.DocumentRelationGateway
	authz     helper.Authorizer
	policy    helper.SharePolicy
	guard     helper.WriteGuard
}

func NewDocumentShareRevoker(documents gateway.DocumentGateway, grants gateway.DocumentGrantGateway, relations gateway.DocumentRelationGateway,
	authz helper.Authorizer, policy helper.SharePolicy, guard helper.WriteGuard) usecase.DocumentShareRevoker {
	return &documentShareRevoker{documents: documents, grants: grants, relations: relations, authz: authz, policy: policy, guard: guard}
}

// Revoke deletes the matching grant's tuple, then its row.
func (s *documentShareRevoker) Revoke(ctx context.Context, in input.DocumentShareRevoker) error {
	g, err := s.policy.NewGrant(in.TenantID, model.ObjectTypeDocument, in.ID, model.ShareInput{Relation: in.Relation, Subject: in.Subject, UserID: in.UserID})
	if err != nil {
		return err
	}
	if err := requireDocumentShare(ctx, s.documents, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
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
