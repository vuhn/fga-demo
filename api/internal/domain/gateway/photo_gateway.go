package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// PhotoGateway is the photos display record.
type PhotoGateway interface {
	// Create inserts the photo and its owner grant in one transaction.
	Create(ctx context.Context, p model.Photo, owner model.Grant) error
	Get(ctx context.Context, id string) (model.Photo, error)
	// ListByIDs loads the given ids, restricted to the tenant and, when
	// albumID is not empty, to that album.
	ListByIDs(ctx context.Context, tenantID string, ids []string, albumID, projectID string) ([]model.Photo, error)
	Rename(ctx context.Context, id, name string) (model.Photo, error)
	// Delete removes the photo and its grants in one transaction.
	Delete(ctx context.Context, id string) error
}

// PhotoGrantGateway is the resource_grants rows of photos.
type PhotoGrantGateway interface {
	List(ctx context.Context, photoID string) ([]model.Grant, error)
	// Upsert inserts the grant or replaces the relation of the existing grant
	// for the same subject, returning the previous grant when one existed.
	Upsert(ctx context.Context, g model.Grant) (prev *model.Grant, err error)
	Delete(ctx context.Context, id string) error
	// Restore puts a grant row back exactly, inserting or updating by id.
	Restore(ctx context.Context, g model.Grant) error
}

// PhotoRelationGateway builds the photo tuples. It is pure.
type PhotoRelationGateway interface {
	// Created links the photo to its tenant, album, and owner.
	Created(p model.Photo) []model.Tuple
	// Grant validates a share on a photo and returns its tuple.
	Grant(g model.Grant) (model.Tuple, error)
}
