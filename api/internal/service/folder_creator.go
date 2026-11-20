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

type folderCreator struct {
	projects  gateway.ProjectGateway
	folders   gateway.FolderGateway
	relations gateway.FolderRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewFolderCreator(projects gateway.ProjectGateway, folders gateway.FolderGateway, relations gateway.FolderRelationGateway, authz helper.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.FolderCreator {
	return &folderCreator{projects: projects, folders: folders, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create needs can_create_folder on the project; a private folder also needs
// can_create_private_folder (internal members); a subfolder also needs
// can_edit on its parent, so an external admin can only nest inside content
// they can already work in.
func (s *folderCreator) Create(ctx context.Context, in input.FolderCreator) (output.FolderCreator, error) {
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.FolderCreator{}, err
	}
	projectID := in.ProjectID
	var parent *model.Folder
	if in.ParentID != nil {
		if err := model.RequireUUID("parent_id", *in.ParentID); err != nil {
			return output.FolderCreator{}, err
		}
		if err := checkFolderDepth(ctx, s.folders, in.TenantID, *in.ParentID); err != nil {
			return output.FolderCreator{}, err
		}
		p, err := loadFolder(ctx, s.folders, in.TenantID, *in.ParentID)
		if err != nil {
			return output.FolderCreator{}, err
		}
		parent = &p
		if projectID != "" && projectID != p.ProjectID {
			return output.FolderCreator{}, model.Invalid("project_id must match the parent's project")
		}
		projectID = p.ProjectID
	} else {
		if err := model.RequireUUID("project_id", projectID); err != nil {
			return output.FolderCreator{}, err
		}
		if _, err := loadProject(ctx, s.projects, in.TenantID, projectID); err != nil {
			return output.FolderCreator{}, err
		}
	}
	if err := s.authz.Require(ctx, in.Actor, "can_create_folder", model.ObjectTypeProject, projectID); err != nil {
		return output.FolderCreator{}, err
	}
	if in.IsPrivate {
		if err := s.authz.Require(ctx, in.Actor, "can_create_private_folder", model.ObjectTypeProject, projectID); err != nil {
			return output.FolderCreator{}, err
		}
	}
	if parent != nil {
		if err := s.authz.Require(ctx, in.Actor, "can_edit", model.ObjectTypeFolder, parent.ID); err != nil {
			return output.FolderCreator{}, err
		}
	}

	f := model.Folder{ID: s.ids.NewID(), TenantID: in.TenantID, ProjectID: projectID, ParentID: in.ParentID, Name: name, CreatedBy: in.Actor, IsPrivate: in.IsPrivate}
	owner := model.OwnerGrant(s.ids.NewID(), in.TenantID, model.ObjectTypeFolder, f.ID, in.Actor)
	if err := s.folders.Create(ctx, f, owner); err != nil {
		return output.FolderCreator{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(f), nil, func(ctx context.Context) error {
		return s.folders.Delete(ctx, f.ID)
	})
	if err != nil {
		return output.FolderCreator{}, err
	}
	got, err := s.folders.Get(ctx, f.ID)
	return output.FolderCreator{Folder: folderOutput(got)}, err
}
