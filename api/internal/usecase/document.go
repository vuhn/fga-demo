package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type DocumentLister interface {
	// List returns every visible document in the tenant, or only those in
	// folderID when it is not empty.
	List(ctx context.Context, actor, tenantID, folderID string) ([]entity.Visible[entity.Document], error)
}

type DocumentGetter interface {
	Get(ctx context.Context, actor, tenantID, id string) (entity.Visible[entity.Document], error)
}

type CreateDocumentInput struct {
	FolderID string
	Name     string
	MimeType string
	ByteSize int64
}

type DocumentCreator interface {
	Create(ctx context.Context, actor, tenantID string, in CreateDocumentInput) (entity.Document, error)
}

type DocumentRenamer interface {
	Rename(ctx context.Context, actor, tenantID, id, name string) (entity.Document, error)
}

type DocumentDeleter interface {
	Delete(ctx context.Context, actor, tenantID, id string) error
}

type DocumentGrantLister interface {
	ListGrants(ctx context.Context, actor, tenantID, id string) ([]entity.Grant, error)
}

// DocumentShareCandidateLister lists the external members a sharer can pick.
type DocumentShareCandidateLister interface {
	ListShareCandidates(ctx context.Context, actor, tenantID, id string) ([]entity.Membership, error)
}

type DocumentSharer interface {
	Share(ctx context.Context, actor, tenantID, id string, in entity.ShareInput) (entity.Grant, error)
}

type DocumentShareRevoker interface {
	Revoke(ctx context.Context, actor, tenantID, id string, in entity.ShareInput) error
}
