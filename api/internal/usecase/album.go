package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type AlbumLister interface {
	List(ctx context.Context, actor, tenantID string) ([]entity.Visible[entity.Album], error)
}

type AlbumGetter interface {
	Get(ctx context.Context, actor, tenantID, id string) (entity.Visible[entity.Album], error)
}

type CreateAlbumInput struct {
	Name      string
	ParentID  *string
	IsPrivate bool
}

type AlbumCreator interface {
	Create(ctx context.Context, actor, tenantID string, in CreateAlbumInput) (entity.Album, error)
}

type AlbumRenamer interface {
	Rename(ctx context.Context, actor, tenantID, id, name string) (entity.Album, error)
}

type AlbumVisibilitySetter interface {
	SetVisibility(ctx context.Context, actor, tenantID, id string, isPrivate bool) (entity.Album, error)
}

type AlbumDeleter interface {
	Delete(ctx context.Context, actor, tenantID, id string) error
}

type AlbumGrantLister interface {
	ListGrants(ctx context.Context, actor, tenantID, id string) ([]entity.Grant, error)
}

// AlbumShareCandidateLister lists the external members a sharer can pick.
type AlbumShareCandidateLister interface {
	ListShareCandidates(ctx context.Context, actor, tenantID, id string) ([]entity.Membership, error)
}

type AlbumSharer interface {
	Share(ctx context.Context, actor, tenantID, id string, in entity.ShareInput) (entity.Grant, error)
}

type AlbumShareRevoker interface {
	Revoke(ctx context.Context, actor, tenantID, id string, in entity.ShareInput) error
}
