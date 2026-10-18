package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type folderLister struct {
	folders gateway.FolderGateway
	authz   usecase.Authorizer
}

func NewFolderLister(folders gateway.FolderGateway, authz usecase.Authorizer) usecase.FolderLister {
	return &folderLister{folders: folders, authz: authz}
}

// List is ListObjects can_view, then a Postgres load of those ids in the
// tenant, with the caller's permissions on each.
func (s *folderLister) List(ctx context.Context, actor, tenantID string) ([]entity.Visible[entity.Folder], error) {
	ids, err := s.authz.ListObjectIDs(ctx, actor, "can_view", entity.ObjectTypeFolder)
	if err != nil || len(ids) == 0 {
		return []entity.Visible[entity.Folder]{}, err
	}
	folders, err := s.folders.ListByIDs(ctx, tenantID, ids)
	if err != nil {
		return nil, err
	}
	return withPerms(ctx, s.authz, actor, entity.ObjectTypeFolder, folders, folderID)
}
