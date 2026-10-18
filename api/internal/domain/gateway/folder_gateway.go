package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// FolderGateway is the folders display record.
type FolderGateway interface {
	// Create inserts the folder and its owner grant in one transaction.
	Create(ctx context.Context, f entity.Folder, owner entity.Grant) error
	Get(ctx context.Context, id string) (entity.Folder, error)
	// ListByIDs loads the given ids, restricted to the tenant.
	ListByIDs(ctx context.Context, tenantID string, ids []string) ([]entity.Folder, error)
	Rename(ctx context.Context, id, name string) (entity.Folder, error)
	// SetPrivate stores the display flag and returns the previous value.
	SetPrivate(ctx context.Context, id string, isPrivate bool) (prev bool, err error)
	// HasChildren reports whether the folder holds subfolders or documents.
	HasChildren(ctx context.Context, id string) (bool, error)
	// Delete removes the folder and its grants in one transaction.
	Delete(ctx context.Context, id string) error
}

// FolderGrantGateway is the resource_grants rows of folders.
type FolderGrantGateway interface {
	List(ctx context.Context, folderID string) ([]entity.Grant, error)
	// Upsert inserts the grant or replaces the relation of the existing grant
	// for the same subject, returning the previous grant when one existed.
	Upsert(ctx context.Context, g entity.Grant) (prev *entity.Grant, err error)
	Delete(ctx context.Context, id string) error
	// Restore puts a grant row back exactly, inserting or updating by id.
	Restore(ctx context.Context, g entity.Grant) error
}

// FolderRelationGateway builds the folder tuples. It is pure.
type FolderRelationGateway interface {
	// Created links the folder to its tenant and owner, then to its parent
	// or (top level) to the tenant as root; a public folder also gets public.
	Created(f entity.Folder) []entity.Tuple
	// Public is the marker that makes the folder externally visible.
	Public(tenantID, folderID string) entity.Tuple
	// Grant validates a share on a folder and returns its tuple.
	Grant(g entity.Grant) (entity.Tuple, error)
}
