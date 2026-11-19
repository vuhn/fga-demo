package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type folderRenamer struct {
	folders gateway.FolderGateway
	authz   usecase.Authorizer
}

func NewFolderRenamer(folders gateway.FolderGateway, authz usecase.Authorizer) usecase.FolderRenamer {
	return &folderRenamer{folders: folders, authz: authz}
}

// Rename requires can_manage: a userset editor can fill a folder but cannot
// rename it.
func (s *folderRenamer) Rename(ctx context.Context, actor, tenantID, id, name string) (model.Folder, error) {
	if _, err := loadFolder(ctx, s.folders, tenantID, id); err != nil {
		return model.Folder{}, err
	}
	name, err := model.CleanName("name", name)
	if err != nil {
		return model.Folder{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_manage", model.ObjectTypeFolder, id); err != nil {
		return model.Folder{}, err
	}
	return s.folders.Rename(ctx, id, name)
}
