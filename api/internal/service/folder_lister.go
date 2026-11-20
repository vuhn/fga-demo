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

type folderLister struct {
	folders gateway.FolderGateway
	authz   helper.Authorizer
}

func NewFolderLister(folders gateway.FolderGateway, authz helper.Authorizer) usecase.FolderLister {
	return &folderLister{folders: folders, authz: authz}
}

// List is ListObjects can_view, then a Postgres load of those ids in the
// tenant, with the caller's permissions on each.
func (s *folderLister) List(ctx context.Context, in input.FolderLister) (output.FolderLister, error) {
	if in.ProjectID != "" {
		if err := model.RequireUUID("project_id", in.ProjectID); err != nil {
			return output.FolderLister{}, err
		}
	}
	ids, err := s.authz.ListObjectIDs(ctx, in.Actor, "can_view", model.ObjectTypeFolder)
	if err != nil || len(ids) == 0 {
		return output.FolderLister{}, err
	}
	folders, err := s.folders.ListByIDs(ctx, in.TenantID, ids, in.ProjectID)
	if err != nil {
		return output.FolderLister{}, err
	}
	got, err := withPerms(ctx, s.authz, in.Actor, model.ObjectTypeFolder, folders, folderID)
	return output.FolderLister{Items: mapVisibleOutput(got, folderOutput)}, err
}
