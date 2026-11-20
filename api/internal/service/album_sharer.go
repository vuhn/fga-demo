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

type albumSharer struct {
	albums    gateway.AlbumGateway
	grants    gateway.AlbumGrantGateway
	relations gateway.AlbumRelationGateway
	authz     helper.Authorizer
	policy    helper.SharePolicy
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewAlbumSharer(albums gateway.AlbumGateway, grants gateway.AlbumGrantGateway, relations gateway.AlbumRelationGateway,
	authz helper.Authorizer, policy helper.SharePolicy, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.AlbumSharer {
	return &albumSharer{albums: albums, grants: grants, relations: relations, authz: authz, policy: policy, guard: guard, ids: ids}
}

// Share grants viewer or editor to an external member or group. A grant to
// a subject that already holds the other relation replaces it.
func (s *albumSharer) Share(ctx context.Context, in input.AlbumSharer) (output.AlbumSharer, error) {
	g, err := s.policy.NewGrant(in.TenantID, model.ObjectTypeAlbum, in.ID, model.ShareInput{Relation: in.Relation, Subject: in.Subject, UserID: in.UserID})
	if err != nil {
		return output.AlbumSharer{}, err
	}
	tuple, err := s.relations.Grant(g) // rejects editor + external_users
	if err != nil {
		return output.AlbumSharer{}, err
	}
	if err := requireAlbumShare(ctx, s.albums, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return output.AlbumSharer{}, err
	}
	if err := s.policy.CheckGrantee(ctx, in.TenantID, g); err != nil {
		return output.AlbumSharer{}, err
	}
	existing, err := s.grants.List(ctx, in.ID)
	if err != nil {
		return output.AlbumSharer{}, err
	}
	for _, e := range existing {
		if !e.SameSubject(g) {
			continue
		}
		if e.Relation == model.RelOwner {
			return output.AlbumSharer{}, model.Invalid("the owner grant cannot be changed")
		}
		if e.Relation == g.Relation {
			return output.AlbumSharer{Grant: grantOutput(e)}, nil
		}
	}

	g.ID = s.ids.NewID()
	prev, err := s.grants.Upsert(ctx, g)
	if err != nil {
		return output.AlbumSharer{}, err
	}
	var replaced []model.Tuple
	if prev != nil {
		old := g
		old.Relation = prev.Relation
		t, err := s.relations.Grant(old)
		if err != nil {
			return output.AlbumSharer{}, err
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
	return output.AlbumSharer{Grant: grantOutput(g)}, err
}
