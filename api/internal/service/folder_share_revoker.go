package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type folderShareRevoker struct {
	folders   gateway.FolderGateway
	grants    gateway.FolderGrantGateway
	relations gateway.FolderRelationGateway
	authz     helper.Authorizer
	policy    helper.SharePolicy
	guard     helper.WriteGuard
}

func NewFolderShareRevoker(folders gateway.FolderGateway, grants gateway.FolderGrantGateway, relations gateway.FolderRelationGateway,
	authz helper.Authorizer, policy helper.SharePolicy, guard helper.WriteGuard) usecase.FolderShareRevoker {
	return &folderShareRevoker{folders: folders, grants: grants, relations: relations, authz: authz, policy: policy, guard: guard}
}

// Revoke deletes the matching grant's tuple, then its row.
func (s *folderShareRevoker) Revoke(ctx context.Context, in input.FolderShareRevoker) error {
	g, err := s.policy.NewGrant(in.TenantID, model.ObjectTypeFolder, in.ID, model.ShareInput{Relation: in.Relation, Subject: in.Subject, UserID: in.UserID})
	if err != nil {
		return err
	}
	if err := requireFolderShare(ctx, s.folders, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
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
