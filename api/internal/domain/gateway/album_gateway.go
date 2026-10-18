package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// AlbumGateway is the albums display record.
type AlbumGateway interface {
	// Create inserts the album and its owner grant in one transaction.
	Create(ctx context.Context, a entity.Album, owner entity.Grant) error
	Get(ctx context.Context, id string) (entity.Album, error)
	// ListByIDs loads the given ids, restricted to the tenant.
	ListByIDs(ctx context.Context, tenantID string, ids []string) ([]entity.Album, error)
	Rename(ctx context.Context, id, name string) (entity.Album, error)
	// SetPrivate stores the display flag and returns the previous value.
	SetPrivate(ctx context.Context, id string, isPrivate bool) (prev bool, err error)
	// HasChildren reports whether the album holds subalbums or photos.
	HasChildren(ctx context.Context, id string) (bool, error)
	// Delete removes the album and its grants in one transaction.
	Delete(ctx context.Context, id string) error
}

// AlbumGrantGateway is the resource_grants rows of albums.
type AlbumGrantGateway interface {
	List(ctx context.Context, albumID string) ([]entity.Grant, error)
	// Upsert inserts the grant or replaces the relation of the existing grant
	// for the same subject, returning the previous grant when one existed.
	Upsert(ctx context.Context, g entity.Grant) (prev *entity.Grant, err error)
	Delete(ctx context.Context, id string) error
	// Restore puts a grant row back exactly, inserting or updating by id.
	Restore(ctx context.Context, g entity.Grant) error
}

// AlbumRelationGateway builds the album tuples. It is pure.
type AlbumRelationGateway interface {
	// Created links the album to its tenant and owner, then to its parent
	// or (top level) to the tenant as root; a public album also gets public.
	Created(a entity.Album) []entity.Tuple
	// Public is the marker that makes the album externally visible.
	Public(tenantID, albumID string) entity.Tuple
	// Grant validates a share on an album and returns its tuple.
	Grant(g entity.Grant) (entity.Tuple, error)
}
