package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// loadAlbum returns the album when it exists in the header tenant.
func loadAlbum(ctx context.Context, albums gateway.AlbumGateway, tenantID, id string) (entity.Album, error) {
	if err := entity.RequireUUID("id", id); err != nil {
		return entity.Album{}, err
	}
	a, err := albums.Get(ctx, id)
	if err != nil {
		return entity.Album{}, err
	}
	if a.TenantID != tenantID {
		return entity.Album{}, entity.ErrNotFound
	}
	return a, nil
}

// checkAlbumDepth walks parent_id in Postgres. The parent must be in the
// header tenant; the chain must be acyclic and at most maxDepth long.
func checkAlbumDepth(ctx context.Context, albums gateway.AlbumGateway, tenantID, parentID string) error {
	seen := map[string]bool{}
	id := &parentID
	for depth := 1; id != nil; depth++ {
		if depth > maxDepth {
			return fmt.Errorf("%w: album depth exceeds %d", entity.ErrConflict, maxDepth)
		}
		if seen[*id] {
			return fmt.Errorf("%w: album cycle", entity.ErrConflict)
		}
		seen[*id] = true
		a, err := albums.Get(ctx, *id)
		if err != nil {
			return err
		}
		if a.TenantID != tenantID {
			return entity.ErrNotFound
		}
		id = a.ParentID
	}
	return nil
}

// requireAlbumShare loads the album in the tenant and requires can_share.
func requireAlbumShare(ctx context.Context, albums gateway.AlbumGateway, authz usecase.Authorizer, actor, tenantID, id string) error {
	if _, err := loadAlbum(ctx, albums, tenantID, id); err != nil {
		return err
	}
	return authz.Require(ctx, actor, "can_share", entity.ObjectTypeAlbum, id)
}

func albumID(a entity.Album) string { return a.ID }
