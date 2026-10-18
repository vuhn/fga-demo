package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type PhotoLister interface {
	// List returns every visible photo in the tenant, or only those in
	// albumID when it is not empty.
	List(ctx context.Context, actor, tenantID, albumID string) ([]entity.Visible[entity.Photo], error)
}

type PhotoGetter interface {
	Get(ctx context.Context, actor, tenantID, id string) (entity.Visible[entity.Photo], error)
}

type CreatePhotoInput struct {
	AlbumID  string
	Name     string
	MimeType string
	ByteSize int64
}

type PhotoCreator interface {
	Create(ctx context.Context, actor, tenantID string, in CreatePhotoInput) (entity.Photo, error)
}

type PhotoRenamer interface {
	Rename(ctx context.Context, actor, tenantID, id, name string) (entity.Photo, error)
}

type PhotoDeleter interface {
	Delete(ctx context.Context, actor, tenantID, id string) error
}

type PhotoGrantLister interface {
	ListGrants(ctx context.Context, actor, tenantID, id string) ([]entity.Grant, error)
}

// PhotoShareCandidateLister lists the external members a sharer can pick.
type PhotoShareCandidateLister interface {
	ListShareCandidates(ctx context.Context, actor, tenantID, id string) ([]entity.Membership, error)
}

type PhotoSharer interface {
	Share(ctx context.Context, actor, tenantID, id string, in entity.ShareInput) (entity.Grant, error)
}

type PhotoShareRevoker interface {
	Revoke(ctx context.Context, actor, tenantID, id string, in entity.ShareInput) error
}
