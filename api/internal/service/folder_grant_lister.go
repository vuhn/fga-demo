package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type folderGrantLister struct {
	folders gateway.FolderGateway
	grants  gateway.FolderGrantGateway
	authz   usecase.Authorizer
}

func NewFolderGrantLister(folders gateway.FolderGateway, grants gateway.FolderGrantGateway, authz usecase.Authorizer) usecase.FolderGrantLister {
	return &folderGrantLister{folders: folders, grants: grants, authz: authz}
}

func (s *folderGrantLister) ListGrants(ctx context.Context, actor, tenantID, id string) ([]entity.Grant, error) {
	if err := requireFolderShare(ctx, s.folders, s.authz, actor, tenantID, id); err != nil {
		return nil, err
	}
	return s.grants.List(ctx, id)
}
