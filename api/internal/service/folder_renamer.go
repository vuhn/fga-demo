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

type folderRenamer struct {
	folders gateway.FolderGateway
	authz   helper.Authorizer
}

func NewFolderRenamer(folders gateway.FolderGateway, authz helper.Authorizer) usecase.FolderRenamer {
	return &folderRenamer{folders: folders, authz: authz}
}

// Rename requires can_manage: a userset editor can fill a folder but cannot
// rename it.
func (s *folderRenamer) Rename(ctx context.Context, in input.FolderRenamer) (output.FolderRenamer, error) {
	if _, err := loadFolder(ctx, s.folders, in.TenantID, in.ID); err != nil {
		return output.FolderRenamer{}, err
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.FolderRenamer{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_manage", model.ObjectTypeFolder, in.ID); err != nil {
		return output.FolderRenamer{}, err
	}
	got, err := s.folders.Rename(ctx, in.ID, name)
	return output.FolderRenamer{Folder: folderOutput(got)}, err
}
