package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type folderGetter struct {
	folders gateway.FolderGateway
	authz   usecase.Authorizer
}

func NewFolderGetter(folders gateway.FolderGateway, authz usecase.Authorizer) usecase.FolderGetter {
	return &folderGetter{folders: folders, authz: authz}
}

// Get is 404 when the caller cannot view the folder.
func (s *folderGetter) Get(ctx context.Context, actor, tenantID, id string) (entity.Visible[entity.Folder], error) {
	f, err := loadFolder(ctx, s.folders, tenantID, id)
	if err != nil {
		return entity.Visible[entity.Folder]{}, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_view", entity.ObjectTypeFolder, id); err != nil {
		return entity.Visible[entity.Folder]{}, err
	}
	vs, err := withPerms(ctx, s.authz, actor, entity.ObjectTypeFolder, []entity.Folder{f}, folderID)
	if err != nil {
		return entity.Visible[entity.Folder]{}, err
	}
	return vs[0], nil
}
