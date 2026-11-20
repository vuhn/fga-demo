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

type folderVisibilitySetter struct {
	folders   gateway.FolderGateway
	relations gateway.FolderRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
}

func NewFolderVisibilitySetter(folders gateway.FolderGateway, relations gateway.FolderRelationGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.FolderVisibilitySetter {
	return &folderVisibilitySetter{folders: folders, relations: relations, authz: authz, guard: guard}
}

// SetVisibility makes a folder private or public. Making it public grants
// external members access (a grant); making it private removes it (a revoke).
func (s *folderVisibilitySetter) SetVisibility(ctx context.Context, in input.FolderVisibilitySetter) (output.FolderVisibilitySetter, error) {
	f, err := loadFolder(ctx, s.folders, in.TenantID, in.ID)
	if err != nil {
		return output.FolderVisibilitySetter{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_change_visibility", model.ObjectTypeFolder, in.ID); err != nil {
		return output.FolderVisibilitySetter{}, err
	}
	if f.IsPrivate == in.IsPrivate {
		return output.FolderVisibilitySetter{Folder: folderOutput(f)}, nil
	}

	public := []model.Tuple{s.relations.Public(in.TenantID, in.ID)}
	makePrivate := func(ctx context.Context) error {
		_, err := s.folders.SetPrivate(ctx, in.ID, true)
		return err
	}
	if in.IsPrivate {
		err = s.guard.Revoke(ctx, public, public, makePrivate)
	} else {
		if _, err := s.folders.SetPrivate(ctx, in.ID, false); err != nil {
			return output.FolderVisibilitySetter{}, err
		}
		err = s.guard.Grant(ctx, public, nil, makePrivate)
	}
	if err != nil {
		return output.FolderVisibilitySetter{}, err
	}
	got, err := s.folders.Get(ctx, in.ID)
	return output.FolderVisibilitySetter{Folder: folderOutput(got)}, err
}
