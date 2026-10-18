package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type albumCreator struct {
	albums    gateway.AlbumGateway
	relations gateway.AlbumRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewAlbumCreator(albums gateway.AlbumGateway, relations gateway.AlbumRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.AlbumCreator {
	return &albumCreator{albums: albums, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create needs can_create_album on the tenant; a private album also needs
// can_create_private_album (internal members); a subalbum also needs
// can_edit on its parent, so an external admin can only nest inside content
// they can already work in.
func (s *albumCreator) Create(ctx context.Context, actor, tenantID string, in usecase.CreateAlbumInput) (entity.Album, error) {
	name, err := entity.CleanName("name", in.Name)
	if err != nil {
		return entity.Album{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_create_album", entity.ObjectTypeTenant, tenantID); err != nil {
		return entity.Album{}, err
	}
	if in.IsPrivate {
		if err := s.authz.Require(ctx, actor, "can_create_private_album", entity.ObjectTypeTenant, tenantID); err != nil {
			return entity.Album{}, err
		}
	}
	if in.ParentID != nil {
		if err := entity.RequireUUID("parent_id", *in.ParentID); err != nil {
			return entity.Album{}, err
		}
		if err := checkAlbumDepth(ctx, s.albums, tenantID, *in.ParentID); err != nil {
			return entity.Album{}, err
		}
		if err := s.authz.Require(ctx, actor, "can_edit", entity.ObjectTypeAlbum, *in.ParentID); err != nil {
			return entity.Album{}, err
		}
	}

	a := entity.Album{ID: s.ids.NewID(), TenantID: tenantID, ParentID: in.ParentID, Name: name, CreatedBy: actor, IsPrivate: in.IsPrivate}
	owner := entity.OwnerGrant(s.ids.NewID(), tenantID, entity.ObjectTypeAlbum, a.ID, actor)
	if err := s.albums.Create(ctx, a, owner); err != nil {
		return entity.Album{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(a), nil, func(ctx context.Context) error {
		return s.albums.Delete(ctx, a.ID)
	})
	if err != nil {
		return entity.Album{}, err
	}
	return s.albums.Get(ctx, a.ID)
}
