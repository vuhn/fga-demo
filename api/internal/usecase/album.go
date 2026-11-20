package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type AlbumLister interface {
	List(ctx context.Context, in input.AlbumLister) (output.AlbumLister, error)
}

type AlbumGetter interface {
	Get(ctx context.Context, in input.AlbumGetter) (output.AlbumGetter, error)
}

type AlbumCreator interface {
	Create(ctx context.Context, in input.AlbumCreator) (output.AlbumCreator, error)
}

type AlbumRenamer interface {
	Rename(ctx context.Context, in input.AlbumRenamer) (output.AlbumRenamer, error)
}

type AlbumVisibilitySetter interface {
	SetVisibility(ctx context.Context, in input.AlbumVisibilitySetter) (output.AlbumVisibilitySetter, error)
}

type AlbumDeleter interface {
	Delete(ctx context.Context, in input.AlbumDeleter) error
}

type AlbumGrantLister interface {
	ListGrants(ctx context.Context, in input.AlbumGrantLister) (output.AlbumGrantLister, error)
}

// AlbumShareCandidateLister lists the external members a sharer can pick.
type AlbumShareCandidateLister interface {
	ListShareCandidates(ctx context.Context, in input.AlbumShareCandidateLister) (output.AlbumShareCandidateLister, error)
}

type AlbumSharer interface {
	Share(ctx context.Context, in input.AlbumSharer) (output.AlbumSharer, error)
}

type AlbumShareRevoker interface {
	Revoke(ctx context.Context, in input.AlbumShareRevoker) error
}
