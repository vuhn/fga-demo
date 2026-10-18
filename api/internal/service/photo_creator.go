package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type photoCreator struct {
	photos    gateway.PhotoGateway
	albums    gateway.AlbumGateway
	relations gateway.PhotoRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewPhotoCreator(photos gateway.PhotoGateway, albums gateway.AlbumGateway, relations gateway.PhotoRelationGateway,
	authz usecase.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.PhotoCreator {
	return &photoCreator{photos: photos, albums: albums, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create is the upload command: can_create_photo on the album. It stores
// metadata only.
func (s *photoCreator) Create(ctx context.Context, actor, tenantID string, in usecase.CreatePhotoInput) (entity.Photo, error) {
	name, err := entity.CleanName("name", in.Name)
	if err != nil {
		return entity.Photo{}, err
	}
	mime := strings.TrimSpace(in.MimeType)
	if n := len(mime); n < 1 || n > 200 {
		return entity.Photo{}, entity.Invalid("mime_type length must be 1 to 200")
	}
	if in.ByteSize < 0 {
		return entity.Photo{}, entity.Invalid("byte_size must be >= 0")
	}
	if _, err := loadAlbum(ctx, s.albums, tenantID, in.AlbumID); err != nil {
		return entity.Photo{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_create_photo", entity.ObjectTypeAlbum, in.AlbumID); err != nil {
		return entity.Photo{}, err
	}

	p := entity.Photo{ID: s.ids.NewID(), TenantID: tenantID, AlbumID: in.AlbumID, Name: name, MimeType: mime, ByteSize: in.ByteSize, CreatedBy: actor}
	owner := entity.OwnerGrant(s.ids.NewID(), tenantID, entity.ObjectTypePhoto, p.ID, actor)
	if err := s.photos.Create(ctx, p, owner); err != nil {
		return entity.Photo{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(p), nil, func(ctx context.Context) error {
		return s.photos.Delete(ctx, p.ID)
	})
	if err != nil {
		return entity.Photo{}, err
	}
	return s.photos.Get(ctx, p.ID)
}
