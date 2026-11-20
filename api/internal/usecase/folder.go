package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type FolderLister interface {
	// List returns visible folders; projectID narrows to one project when not empty.
	List(ctx context.Context, in input.FolderLister) (output.FolderLister, error)
}

type FolderGetter interface {
	Get(ctx context.Context, in input.FolderGetter) (output.FolderGetter, error)
}

type FolderCreator interface {
	Create(ctx context.Context, in input.FolderCreator) (output.FolderCreator, error)
}

type FolderRenamer interface {
	Rename(ctx context.Context, in input.FolderRenamer) (output.FolderRenamer, error)
}

type FolderVisibilitySetter interface {
	SetVisibility(ctx context.Context, in input.FolderVisibilitySetter) (output.FolderVisibilitySetter, error)
}

type FolderDeleter interface {
	Delete(ctx context.Context, in input.FolderDeleter) error
}

type FolderGrantLister interface {
	ListGrants(ctx context.Context, in input.FolderGrantLister) (output.FolderGrantLister, error)
}

// FolderShareCandidateLister lists the external members a sharer can pick.
type FolderShareCandidateLister interface {
	ListShareCandidates(ctx context.Context, in input.FolderShareCandidateLister) (output.FolderShareCandidateLister, error)
}

type FolderSharer interface {
	Share(ctx context.Context, in input.FolderSharer) (output.FolderSharer, error)
}

type FolderShareRevoker interface {
	Revoke(ctx context.Context, in input.FolderShareRevoker) error
}
