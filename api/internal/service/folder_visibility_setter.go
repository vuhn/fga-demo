package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type folderVisibilitySetter struct {
	folders   gateway.FolderGateway
	relations gateway.FolderRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewFolderVisibilitySetter(folders gateway.FolderGateway, relations gateway.FolderRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.FolderVisibilitySetter {
	return &folderVisibilitySetter{folders: folders, relations: relations, authz: authz, guard: guard}
}

// SetVisibility makes a folder private or public. Making it public grants
// external members access (a grant); making it private removes it (a revoke).
func (s *folderVisibilitySetter) SetVisibility(ctx context.Context, actor, tenantID, id string, isPrivate bool) (entity.Folder, error) {
	f, err := loadFolder(ctx, s.folders, tenantID, id)
	if err != nil {
		return entity.Folder{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_change_visibility", entity.ObjectTypeFolder, id); err != nil {
		return entity.Folder{}, err
	}
	if f.IsPrivate == isPrivate {
		return f, nil
	}

	public := []entity.Tuple{s.relations.Public(tenantID, id)}
	makePrivate := func(ctx context.Context) error {
		_, err := s.folders.SetPrivate(ctx, id, true)
		return err
	}
	if isPrivate {
		err = s.guard.Revoke(ctx, public, public, makePrivate)
	} else {
		if _, err := s.folders.SetPrivate(ctx, id, false); err != nil {
			return entity.Folder{}, err
		}
		err = s.guard.Grant(ctx, public, nil, makePrivate)
	}
	if err != nil {
		return entity.Folder{}, err
	}
	return s.folders.Get(ctx, id)
}
