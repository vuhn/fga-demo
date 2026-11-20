package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type DocumentLister interface {
	// List returns every visible document in the tenant, or only those in
	// folderID when it is not empty.
	List(ctx context.Context, in input.DocumentLister) (output.DocumentLister, error)
}

type DocumentGetter interface {
	Get(ctx context.Context, in input.DocumentGetter) (output.DocumentGetter, error)
}

type DocumentCreator interface {
	Create(ctx context.Context, in input.DocumentCreator) (output.DocumentCreator, error)
}

type DocumentRenamer interface {
	Rename(ctx context.Context, in input.DocumentRenamer) (output.DocumentRenamer, error)
}

type DocumentDeleter interface {
	Delete(ctx context.Context, in input.DocumentDeleter) error
}

type DocumentGrantLister interface {
	ListGrants(ctx context.Context, in input.DocumentGrantLister) (output.DocumentGrantLister, error)
}

// DocumentShareCandidateLister lists the external members a sharer can pick.
type DocumentShareCandidateLister interface {
	ListShareCandidates(ctx context.Context, in input.DocumentShareCandidateLister) (output.DocumentShareCandidateLister, error)
}

type DocumentSharer interface {
	Share(ctx context.Context, in input.DocumentSharer) (output.DocumentSharer, error)
}

type DocumentShareRevoker interface {
	Revoke(ctx context.Context, in input.DocumentShareRevoker) error
}
