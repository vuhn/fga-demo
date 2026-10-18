package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type folderCreator struct {
	folders   gateway.FolderGateway
	relations gateway.FolderRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewFolderCreator(folders gateway.FolderGateway, relations gateway.FolderRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.FolderCreator {
	return &folderCreator{folders: folders, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create needs can_create_folder on the tenant; a private folder also needs
// can_create_private_folder (internal members); a subfolder also needs
// can_edit on its parent, so an external admin can only nest inside content
// they can already work in.
func (s *folderCreator) Create(ctx context.Context, actor, tenantID string, in usecase.CreateFolderInput) (entity.Folder, error) {
	name, err := entity.CleanName("name", in.Name)
	if err != nil {
		return entity.Folder{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_create_folder", entity.ObjectTypeTenant, tenantID); err != nil {
		return entity.Folder{}, err
	}
	if in.IsPrivate {
		if err := s.authz.Require(ctx, actor, "can_create_private_folder", entity.ObjectTypeTenant, tenantID); err != nil {
			return entity.Folder{}, err
		}
	}
	if in.ParentID != nil {
		if err := entity.RequireUUID("parent_id", *in.ParentID); err != nil {
			return entity.Folder{}, err
		}
		if err := checkFolderDepth(ctx, s.folders, tenantID, *in.ParentID); err != nil {
			return entity.Folder{}, err
		}
		if err := s.authz.Require(ctx, actor, "can_edit", entity.ObjectTypeFolder, *in.ParentID); err != nil {
			return entity.Folder{}, err
		}
	}

	f := entity.Folder{ID: s.ids.NewID(), TenantID: tenantID, ParentID: in.ParentID, Name: name, CreatedBy: actor, IsPrivate: in.IsPrivate}
	owner := entity.OwnerGrant(s.ids.NewID(), tenantID, entity.ObjectTypeFolder, f.ID, actor)
	if err := s.folders.Create(ctx, f, owner); err != nil {
		return entity.Folder{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(f), nil, func(ctx context.Context) error {
		return s.folders.Delete(ctx, f.ID)
	})
	if err != nil {
		return entity.Folder{}, err
	}
	return s.folders.Get(ctx, f.ID)
}
