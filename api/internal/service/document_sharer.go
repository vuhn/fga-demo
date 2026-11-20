package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type documentSharer struct {
	documents gateway.DocumentGateway
	grants    gateway.DocumentGrantGateway
	relations gateway.DocumentRelationGateway
	authz     helper.Authorizer
	policy    helper.SharePolicy
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewDocumentSharer(documents gateway.DocumentGateway, grants gateway.DocumentGrantGateway, relations gateway.DocumentRelationGateway,
	authz helper.Authorizer, policy helper.SharePolicy, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.DocumentSharer {
	return &documentSharer{documents: documents, grants: grants, relations: relations, authz: authz, policy: policy, guard: guard, ids: ids}
}

// Share grants viewer or editor to an external member or group. A grant to
// a subject that already holds the other relation replaces it.
func (s *documentSharer) Share(ctx context.Context, in input.DocumentSharer) (output.DocumentSharer, error) {
	g, err := s.policy.NewGrant(in.TenantID, model.ObjectTypeDocument, in.ID, model.ShareInput{Relation: in.Relation, Subject: in.Subject, UserID: in.UserID})
	if err != nil {
		return output.DocumentSharer{}, err
	}
	tuple, err := s.relations.Grant(g) // rejects editor + external_users
	if err != nil {
		return output.DocumentSharer{}, err
	}
	if err := requireDocumentShare(ctx, s.documents, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return output.DocumentSharer{}, err
	}
	if err := s.policy.CheckGrantee(ctx, in.TenantID, g); err != nil {
		return output.DocumentSharer{}, err
	}
	existing, err := s.grants.List(ctx, in.ID)
	if err != nil {
		return output.DocumentSharer{}, err
	}
	for _, e := range existing {
		if !e.SameSubject(g) {
			continue
		}
		if e.Relation == model.RelOwner {
			return output.DocumentSharer{}, model.Invalid("the owner grant cannot be changed")
		}
		if e.Relation == g.Relation {
			return output.DocumentSharer{Grant: grantOutput(e)}, nil
		}
	}

	g.ID = s.ids.NewID()
	prev, err := s.grants.Upsert(ctx, g)
	if err != nil {
		return output.DocumentSharer{}, err
	}
	var replaced []model.Tuple
	if prev != nil {
		old := g
		old.Relation = prev.Relation
		t, err := s.relations.Grant(old)
		if err != nil {
			return output.DocumentSharer{}, err
		}
		replaced = append(replaced, t)
		g.ID = prev.ID
	}
	err = s.guard.Grant(ctx, []model.Tuple{tuple}, replaced, func(ctx context.Context) error {
		if prev == nil {
			return s.grants.Delete(ctx, g.ID)
		}
		return s.grants.Restore(ctx, *prev)
	})
	return output.DocumentSharer{Grant: grantOutput(g)}, err
}
