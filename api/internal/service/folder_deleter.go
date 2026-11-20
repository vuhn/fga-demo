package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type folderDeleter struct {
	folders gateway.FolderGateway
	authz   helper.Authorizer
	guard   helper.WriteGuard
}

func NewFolderDeleter(folders gateway.FolderGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.FolderDeleter {
	return &folderDeleter{folders: folders, authz: authz, guard: guard}
}

// Delete requires can_delete and an empty folder. It removes every tuple on
// the folder, then the row and its grants.
func (s *folderDeleter) Delete(ctx context.Context, in input.FolderDeleter) error {
	if _, err := loadFolder(ctx, s.folders, in.TenantID, in.ID); err != nil {
		return err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_delete", model.ObjectTypeFolder, in.ID); err != nil {
		return err
	}
	has, err := s.folders.HasChildren(ctx, in.ID)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("%w: folder still has children", model.ErrConflict)
	}
	return s.guard.RevokeObject(ctx, model.ObjectTypeFolder, in.ID, func(ctx context.Context) error {
		return s.folders.Delete(ctx, in.ID)
	})
}
