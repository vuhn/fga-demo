package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
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
func (s *folderRenamer) Rename(ctx context.Context, actor, tenantID, id, name string) (entity.Folder, error) {
	if _, err := loadFolder(ctx, s.folders, tenantID, id); err != nil {
		return entity.Folder{}, err
	}
	name, err := entity.CleanName("name", name)
	if err != nil {
		return entity.Folder{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_manage", entity.ObjectTypeFolder, id); err != nil {
		return entity.Folder{}, err
	}
	return s.folders.Rename(ctx, id, name)
}
