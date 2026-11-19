package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// loadFolder returns the folder when it exists in the header tenant.
func loadFolder(ctx context.Context, folders gateway.FolderGateway, tenantID, id string) (model.Folder, error) {
	if err := model.RequireUUID("id", id); err != nil {
		return model.Folder{}, err
	}
	f, err := folders.Get(ctx, id)
	if err != nil {
		return model.Folder{}, err
	}
	if f.TenantID != tenantID {
		return model.Folder{}, model.ErrNotFound
	}
	return f, nil
}

// checkFolderDepth walks parent_id in Postgres. The parent must be in the
// header tenant; the chain must be acyclic and at most maxDepth long.
func checkFolderDepth(ctx context.Context, folders gateway.FolderGateway, tenantID, parentID string) error {
	seen := map[string]bool{}
	id := &parentID
	for depth := 1; id != nil; depth++ {
		if depth > maxDepth {
			return fmt.Errorf("%w: folder depth exceeds %d", model.ErrConflict, maxDepth)
		}
		if seen[*id] {
			return fmt.Errorf("%w: folder cycle", model.ErrConflict)
		}
		seen[*id] = true
		f, err := folders.Get(ctx, *id)
		if err != nil {
			return err
		}
		if f.TenantID != tenantID {
			return model.ErrNotFound
		}
		id = f.ParentID
	}
	return nil
}

// requireFolderShare loads the folder in the tenant and requires can_share.
func requireFolderShare(ctx context.Context, folders gateway.FolderGateway, authz usecase.Authorizer, actor, tenantID, id string) error {
	if _, err := loadFolder(ctx, folders, tenantID, id); err != nil {
		return err
	}
	return authz.Require(ctx, actor, "can_share", model.ObjectTypeFolder, id)
}

func folderID(f model.Folder) string { return f.ID }
