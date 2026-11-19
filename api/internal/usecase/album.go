package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type AlbumLister interface {
	List(ctx context.Context, actor, tenantID, projectID string) ([]model.Visible[model.Album], error)
}

type AlbumGetter interface {
	Get(ctx context.Context, actor, tenantID, id string) (model.Visible[model.Album], error)
}

type CreateAlbumInput struct {
	Name string
	// ProjectID is required for a top-level album; a subalbum takes its parent's.
	ProjectID string
	ParentID  *string
	IsPrivate bool
}

type AlbumCreator interface {
	Create(ctx context.Context, actor, tenantID string, in CreateAlbumInput) (model.Album, error)
}

type AlbumRenamer interface {
	Rename(ctx context.Context, actor, tenantID, id, name string) (model.Album, error)
}

type AlbumVisibilitySetter interface {
	SetVisibility(ctx context.Context, actor, tenantID, id string, isPrivate bool) (model.Album, error)
}

type AlbumDeleter interface {
	Delete(ctx context.Context, actor, tenantID, id string) error
}

type AlbumGrantLister interface {
	ListGrants(ctx context.Context, actor, tenantID, id string) ([]model.Grant, error)
}

// AlbumShareCandidateLister lists the external members a sharer can pick.
type AlbumShareCandidateLister interface {
	ListShareCandidates(ctx context.Context, actor, tenantID, id string) ([]model.Membership, error)
}

type AlbumSharer interface {
	Share(ctx context.Context, actor, tenantID, id string, in model.ShareInput) (model.Grant, error)
}

type AlbumShareRevoker interface {
	Revoke(ctx context.Context, actor, tenantID, id string, in model.ShareInput) error
}
