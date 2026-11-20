package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type folderGrantLister struct {
	folders gateway.FolderGateway
	grants  gateway.FolderGrantGateway
	authz   helper.Authorizer
}

func NewFolderGrantLister(folders gateway.FolderGateway, grants gateway.FolderGrantGateway, authz helper.Authorizer) usecase.FolderGrantLister {
	return &folderGrantLister{folders: folders, grants: grants, authz: authz}
}

func (s *folderGrantLister) ListGrants(ctx context.Context, in input.FolderGrantLister) (output.FolderGrantLister, error) {
	if err := requireFolderShare(ctx, s.folders, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return output.FolderGrantLister{}, err
	}
	got, err := s.grants.List(ctx, in.ID)
	return output.FolderGrantLister{Items: mapOutput(got, grantOutput)}, err
}
