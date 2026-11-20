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

type folderShareCandidateLister struct {
	folders gateway.FolderGateway
	members gateway.MembershipGateway
	authz   helper.Authorizer
}

func NewFolderShareCandidateLister(folders gateway.FolderGateway, members gateway.MembershipGateway, authz helper.Authorizer) usecase.FolderShareCandidateLister {
	return &folderShareCandidateLister{folders: folders, members: members, authz: authz}
}

// ListShareCandidates lists the external members of the folder's tenant. It
// needs can_share on the folder, not member management, so owners can share
// without seeing the full roster.
func (s *folderShareCandidateLister) ListShareCandidates(ctx context.Context, in input.FolderShareCandidateLister) (output.FolderShareCandidateLister, error) {
	if err := requireFolderShare(ctx, s.folders, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return output.FolderShareCandidateLister{}, err
	}
	got, err := s.members.List(ctx, in.TenantID, model.AffiliationExternal)
	return output.FolderShareCandidateLister{Items: mapOutput(got, membershipOutput)}, err
}
