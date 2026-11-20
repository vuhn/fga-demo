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

type folderGetter struct {
	folders gateway.FolderGateway
	authz   helper.Authorizer
}

func NewFolderGetter(folders gateway.FolderGateway, authz helper.Authorizer) usecase.FolderGetter {
	return &folderGetter{folders: folders, authz: authz}
}

// Get is 404 when the caller cannot view the folder.
func (s *folderGetter) Get(ctx context.Context, in input.FolderGetter) (output.FolderGetter, error) {
	f, err := loadFolder(ctx, s.folders, in.TenantID, in.ID)
	if err != nil {
		return output.FolderGetter{}, err
	}
	if err := s.authz.RequireVisible(ctx, in.Actor, "can_view", model.ObjectTypeFolder, in.ID); err != nil {
		return output.FolderGetter{}, err
	}
	vs, err := withPerms(ctx, s.authz, in.Actor, model.ObjectTypeFolder, []model.Folder{f}, folderID)
	if err != nil {
		return output.FolderGetter{}, err
	}
	return output.FolderGetter{Visible: visibleOutput(vs[0], folderOutput)}, nil
}
