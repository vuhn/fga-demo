package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// PhotoGateway is the photos display record.
type PhotoGateway interface {
	// Create inserts the photo and its owner grant in one transaction.
	Create(ctx context.Context, p entity.Photo, owner entity.Grant) error
	Get(ctx context.Context, id string) (entity.Photo, error)
	// ListByIDs loads the given ids, restricted to the tenant and, when
	// albumID is not empty, to that album.
	ListByIDs(ctx context.Context, tenantID string, ids []string, albumID string) ([]entity.Photo, error)
	Rename(ctx context.Context, id, name string) (entity.Photo, error)
	// Delete removes the photo and its grants in one transaction.
	Delete(ctx context.Context, id string) error
}

// PhotoGrantGateway is the resource_grants rows of photos.
type PhotoGrantGateway interface {
	List(ctx context.Context, photoID string) ([]entity.Grant, error)
	// Upsert inserts the grant or replaces the relation of the existing grant
	// for the same subject, returning the previous grant when one existed.
	Upsert(ctx context.Context, g entity.Grant) (prev *entity.Grant, err error)
	Delete(ctx context.Context, id string) error
	// Restore puts a grant row back exactly, inserting or updating by id.
	Restore(ctx context.Context, g entity.Grant) error
}

// PhotoRelationGateway builds the photo tuples. It is pure.
type PhotoRelationGateway interface {
	// Created links the photo to its tenant, album, and owner.
	Created(p entity.Photo) []entity.Tuple
	// Grant validates a share on a photo and returns its tuple.
	Grant(g entity.Grant) (entity.Tuple, error)
}
