package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type albumCreator struct {
	projects  gateway.ProjectGateway
	albums    gateway.AlbumGateway
	relations gateway.AlbumRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewAlbumCreator(projects gateway.ProjectGateway, albums gateway.AlbumGateway, relations gateway.AlbumRelationGateway, authz helper.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.AlbumCreator {
	return &albumCreator{projects: projects, albums: albums, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create needs can_create_album on the project; a private album also needs
// can_create_private_album (internal members); a subalbum also needs
// can_edit on its parent, so an external admin can only nest inside content
// they can already work in.
func (s *albumCreator) Create(ctx context.Context, in input.AlbumCreator) (output.AlbumCreator, error) {
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.AlbumCreator{}, err
	}
	projectID := in.ProjectID
	var parent *model.Album
	if in.ParentID != nil {
		if err := model.RequireUUID("parent_id", *in.ParentID); err != nil {
			return output.AlbumCreator{}, err
		}
		if err := checkAlbumDepth(ctx, s.albums, in.TenantID, *in.ParentID); err != nil {
			return output.AlbumCreator{}, err
		}
		p, err := loadAlbum(ctx, s.albums, in.TenantID, *in.ParentID)
		if err != nil {
			return output.AlbumCreator{}, err
		}
		parent = &p
		if projectID != "" && projectID != p.ProjectID {
			return output.AlbumCreator{}, model.Invalid("project_id must match the parent's project")
		}
		projectID = p.ProjectID
	} else {
		if err := model.RequireUUID("project_id", projectID); err != nil {
			return output.AlbumCreator{}, err
		}
		if _, err := loadProject(ctx, s.projects, in.TenantID, projectID); err != nil {
			return output.AlbumCreator{}, err
		}
	}
	if err := s.authz.Require(ctx, in.Actor, "can_create_album", model.ObjectTypeProject, projectID); err != nil {
		return output.AlbumCreator{}, err
	}
	if in.IsPrivate {
		if err := s.authz.Require(ctx, in.Actor, "can_create_private_album", model.ObjectTypeProject, projectID); err != nil {
			return output.AlbumCreator{}, err
		}
	}
	if parent != nil {
		if err := s.authz.Require(ctx, in.Actor, "can_edit", model.ObjectTypeAlbum, parent.ID); err != nil {
			return output.AlbumCreator{}, err
		}
	}

	a := model.Album{ID: s.ids.NewID(), TenantID: in.TenantID, ProjectID: projectID, ParentID: in.ParentID, Name: name, CreatedBy: in.Actor, IsPrivate: in.IsPrivate}
	owner := model.OwnerGrant(s.ids.NewID(), in.TenantID, model.ObjectTypeAlbum, a.ID, in.Actor)
	if err := s.albums.Create(ctx, a, owner); err != nil {
		return output.AlbumCreator{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(a), nil, func(ctx context.Context) error {
		return s.albums.Delete(ctx, a.ID)
	})
	if err != nil {
		return output.AlbumCreator{}, err
	}
	got, err := s.albums.Get(ctx, a.ID)
	return output.AlbumCreator{Album: albumOutput(got)}, err
}
