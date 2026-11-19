package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
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
func (s *folderGetter) Get(ctx context.Context, actor, tenantID, id string) (model.Visible[model.Folder], error) {
	f, err := loadFolder(ctx, s.folders, tenantID, id)
	if err != nil {
		return model.Visible[model.Folder]{}, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_view", model.ObjectTypeFolder, id); err != nil {
		return model.Visible[model.Folder]{}, err
	}
	vs, err := withPerms(ctx, s.authz, actor, model.ObjectTypeFolder, []model.Folder{f}, folderID)
	if err != nil {
		return model.Visible[model.Folder]{}, err
	}
	return vs[0], nil
}
