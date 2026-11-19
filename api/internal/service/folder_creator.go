package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type folderCreator struct {
	projects  gateway.ProjectGateway
	folders   gateway.FolderGateway
	relations gateway.FolderRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewFolderCreator(projects gateway.ProjectGateway, folders gateway.FolderGateway, relations gateway.FolderRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.FolderCreator {
	return &folderCreator{projects: projects, folders: folders, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create needs can_create_folder on the project; a private folder also needs
// can_create_private_folder (internal members); a subfolder also needs
// can_edit on its parent, so an external admin can only nest inside content
// they can already work in.
func (s *folderCreator) Create(ctx context.Context, actor, tenantID string, in usecase.CreateFolderInput) (model.Folder, error) {
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return model.Folder{}, err
	}
	projectID := in.ProjectID
	var parent *model.Folder
	if in.ParentID != nil {
		if err := model.RequireUUID("parent_id", *in.ParentID); err != nil {
			return model.Folder{}, err
		}
		if err := checkFolderDepth(ctx, s.folders, tenantID, *in.ParentID); err != nil {
			return model.Folder{}, err
		}
		p, err := loadFolder(ctx, s.folders, tenantID, *in.ParentID)
		if err != nil {
			return model.Folder{}, err
		}
		parent = &p
		if projectID != "" && projectID != p.ProjectID {
			return model.Folder{}, model.Invalid("project_id must match the parent's project")
		}
		projectID = p.ProjectID
	} else {
		if err := model.RequireUUID("project_id", projectID); err != nil {
			return model.Folder{}, err
		}
		if _, err := loadProject(ctx, s.projects, tenantID, projectID); err != nil {
			return model.Folder{}, err
		}
	}
	if err := s.authz.Require(ctx, actor, "can_create_folder", model.ObjectTypeProject, projectID); err != nil {
		return model.Folder{}, err
	}
	if in.IsPrivate {
		if err := s.authz.Require(ctx, actor, "can_create_private_folder", model.ObjectTypeProject, projectID); err != nil {
			return model.Folder{}, err
		}
	}
	if parent != nil {
		if err := s.authz.Require(ctx, actor, "can_edit", model.ObjectTypeFolder, parent.ID); err != nil {
			return model.Folder{}, err
		}
	}

	f := model.Folder{ID: s.ids.NewID(), TenantID: tenantID, ProjectID: projectID, ParentID: in.ParentID, Name: name, CreatedBy: actor, IsPrivate: in.IsPrivate}
	owner := model.OwnerGrant(s.ids.NewID(), tenantID, model.ObjectTypeFolder, f.ID, actor)
	if err := s.folders.Create(ctx, f, owner); err != nil {
		return model.Folder{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(f), nil, func(ctx context.Context) error {
		return s.folders.Delete(ctx, f.ID)
	})
	if err != nil {
		return model.Folder{}, err
	}
	return s.folders.Get(ctx, f.ID)
}
