package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type PhotoLister interface {
	// List returns every visible photo in the tenant, or only those in
	// albumID when it is not empty.
	List(ctx context.Context, in input.PhotoLister) (output.PhotoLister, error)
}

type PhotoGetter interface {
	Get(ctx context.Context, in input.PhotoGetter) (output.PhotoGetter, error)
}

type PhotoCreator interface {
	Create(ctx context.Context, in input.PhotoCreator) (output.PhotoCreator, error)
}

type PhotoRenamer interface {
	Rename(ctx context.Context, in input.PhotoRenamer) (output.PhotoRenamer, error)
}

type PhotoDeleter interface {
	Delete(ctx context.Context, in input.PhotoDeleter) error
}

type PhotoGrantLister interface {
	ListGrants(ctx context.Context, in input.PhotoGrantLister) (output.PhotoGrantLister, error)
}

// PhotoShareCandidateLister lists the external members a sharer can pick.
type PhotoShareCandidateLister interface {
	ListShareCandidates(ctx context.Context, in input.PhotoShareCandidateLister) (output.PhotoShareCandidateLister, error)
}

type PhotoSharer interface {
	Share(ctx context.Context, in input.PhotoSharer) (output.PhotoSharer, error)
}

type PhotoShareRevoker interface {
	Revoke(ctx context.Context, in input.PhotoShareRevoker) error
}
