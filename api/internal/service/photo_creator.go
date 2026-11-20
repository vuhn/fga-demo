package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type photoCreator struct {
	photos    gateway.PhotoGateway
	albums    gateway.AlbumGateway
	relations gateway.PhotoRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewPhotoCreator(photos gateway.PhotoGateway, albums gateway.AlbumGateway, relations gateway.PhotoRelationGateway,
	authz helper.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.PhotoCreator {
	return &photoCreator{photos: photos, albums: albums, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create is the upload command: can_create_photo on the album. It stores
// metadata only.
func (s *photoCreator) Create(ctx context.Context, in input.PhotoCreator) (output.PhotoCreator, error) {
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.PhotoCreator{}, err
	}
	mime := strings.TrimSpace(in.MimeType)
	if n := len(mime); n < 1 || n > 200 {
		return output.PhotoCreator{}, model.Invalid("mime_type length must be 1 to 200")
	}
	if in.ByteSize < 0 {
		return output.PhotoCreator{}, model.Invalid("byte_size must be >= 0")
	}
	album, err := loadAlbum(ctx, s.albums, in.TenantID, in.AlbumID)
	if err != nil {
		return output.PhotoCreator{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_create_photo", model.ObjectTypeAlbum, in.AlbumID); err != nil {
		return output.PhotoCreator{}, err
	}

	p := model.Photo{ID: s.ids.NewID(), TenantID: in.TenantID, ProjectID: album.ProjectID, AlbumID: in.AlbumID, Name: name, MimeType: mime, ByteSize: in.ByteSize, CreatedBy: in.Actor}
	owner := model.OwnerGrant(s.ids.NewID(), in.TenantID, model.ObjectTypePhoto, p.ID, in.Actor)
	if err := s.photos.Create(ctx, p, owner); err != nil {
		return output.PhotoCreator{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(p), nil, func(ctx context.Context) error {
		return s.photos.Delete(ctx, p.ID)
	})
	if err != nil {
		return output.PhotoCreator{}, err
	}
	got, err := s.photos.Get(ctx, p.ID)
	return output.PhotoCreator{Photo: photoOutput(got)}, err
}
