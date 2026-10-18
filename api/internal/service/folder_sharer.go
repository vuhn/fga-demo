package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type folderSharer struct {
	folders   gateway.FolderGateway
	grants    gateway.FolderGrantGateway
	relations gateway.FolderRelationGateway
	authz     usecase.Authorizer
	policy    helper.SharePolicy
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewFolderSharer(folders gateway.FolderGateway, grants gateway.FolderGrantGateway, relations gateway.FolderRelationGateway,
	authz usecase.Authorizer, policy helper.SharePolicy, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.FolderSharer {
	return &folderSharer{folders: folders, grants: grants, relations: relations, authz: authz, policy: policy, guard: guard, ids: ids}
}

// Share grants viewer or editor to an external member or group. A grant to
// a subject that already holds the other relation replaces it.
func (s *folderSharer) Share(ctx context.Context, actor, tenantID, id string, in entity.ShareInput) (entity.Grant, error) {
	g, err := s.policy.NewGrant(tenantID, entity.ObjectTypeFolder, id, in)
	if err != nil {
		return entity.Grant{}, err
	}
	tuple, err := s.relations.Grant(g) // rejects editor + external_users
	if err != nil {
		return entity.Grant{}, err
	}
	if err := requireFolderShare(ctx, s.folders, s.authz, actor, tenantID, id); err != nil {
		return entity.Grant{}, err
	}
	if err := s.policy.CheckGrantee(ctx, tenantID, g); err != nil {
		return entity.Grant{}, err
	}
	existing, err := s.grants.List(ctx, id)
	if err != nil {
		return entity.Grant{}, err
	}
	for _, e := range existing {
		if !e.SameSubject(g) {
			continue
		}
		if e.Relation == entity.RelOwner {
			return entity.Grant{}, entity.Invalid("the owner grant cannot be changed")
		}
		if e.Relation == g.Relation {
			return e, nil
		}
	}

	g.ID = s.ids.NewID()
	prev, err := s.grants.Upsert(ctx, g)
	if err != nil {
		return entity.Grant{}, err
	}
	var replaced []entity.Tuple
	if prev != nil {
		old := g
		old.Relation = prev.Relation
		t, err := s.relations.Grant(old)
		if err != nil {
			return entity.Grant{}, err
		}
		replaced = append(replaced, t)
		g.ID = prev.ID
	}
	err = s.guard.Grant(ctx, []entity.Tuple{tuple}, replaced, func(ctx context.Context) error {
		if prev == nil {
			return s.grants.Delete(ctx, g.ID)
		}
		return s.grants.Restore(ctx, *prev)
	})
	return g, err
}
