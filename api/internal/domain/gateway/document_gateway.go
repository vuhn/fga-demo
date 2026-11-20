package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// DocumentGateway is the documents display record.
type DocumentGateway interface {
	// Create inserts the document and its owner grant in one transaction.
	Create(ctx context.Context, d model.Document, owner model.Grant) error
	Get(ctx context.Context, id string) (model.Document, error)
	// ListByIDs loads the given ids, restricted to the tenant and, when
	// folderID is not empty, to that folder.
	ListByIDs(ctx context.Context, p model.ListDocumentsParams) ([]model.Document, error)
	Rename(ctx context.Context, id, name string) (model.Document, error)
	// Delete removes the document and its grants in one transaction.
	Delete(ctx context.Context, id string) error
}

// DocumentGrantGateway is the resource_grants rows of documents.
type DocumentGrantGateway interface {
	List(ctx context.Context, documentID string) ([]model.Grant, error)
	// Upsert inserts the grant or replaces the relation of the existing grant
	// for the same subject, returning the previous grant when one existed.
	Upsert(ctx context.Context, g model.Grant) (prev *model.Grant, err error)
	Delete(ctx context.Context, id string) error
	// Restore puts a grant row back exactly, inserting or updating by id.
	Restore(ctx context.Context, g model.Grant) error
}

// DocumentRelationGateway builds the document tuples. It is pure.
type DocumentRelationGateway interface {
	// Created links the document to its tenant, folder, and owner.
	Created(d model.Document) []model.Tuple
	// Grant validates a share on a document and returns its tuple.
	Grant(g model.Grant) (model.Tuple, error)
}
