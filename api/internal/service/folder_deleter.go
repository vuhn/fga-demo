package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type folderDeleter struct {
	folders gateway.FolderGateway
	authz   usecase.Authorizer
	guard   helper.WriteGuard
}

func NewFolderDeleter(folders gateway.FolderGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.FolderDeleter {
	return &folderDeleter{folders: folders, authz: authz, guard: guard}
}

// Delete requires can_delete and an empty folder. It removes every tuple on
// the folder, then the row and its grants.
func (s *folderDeleter) Delete(ctx context.Context, actor, tenantID, id string) error {
	if _, err := loadFolder(ctx, s.folders, tenantID, id); err != nil {
		return err
	}
	if err := s.authz.Require(ctx, actor, "can_delete", entity.ObjectTypeFolder, id); err != nil {
		return err
	}
	has, err := s.folders.HasChildren(ctx, id)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("%w: folder still has children", entity.ErrConflict)
	}
	return s.guard.RevokeObject(ctx, entity.ObjectTypeFolder, id, func(ctx context.Context) error {
		return s.folders.Delete(ctx, id)
	})
}
