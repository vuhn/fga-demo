package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type folderShareCandidateLister struct {
	folders gateway.FolderGateway
	members gateway.MembershipGateway
	authz   usecase.Authorizer
}

func NewFolderShareCandidateLister(folders gateway.FolderGateway, members gateway.MembershipGateway, authz usecase.Authorizer) usecase.FolderShareCandidateLister {
	return &folderShareCandidateLister{folders: folders, members: members, authz: authz}
}

// ListShareCandidates lists the external members of the folder's tenant. It
// needs can_share on the folder, not member management, so owners can share
// without seeing the full roster.
func (s *folderShareCandidateLister) ListShareCandidates(ctx context.Context, actor, tenantID, id string) ([]entity.Membership, error) {
	if err := requireFolderShare(ctx, s.folders, s.authz, actor, tenantID, id); err != nil {
		return nil, err
	}
	return s.members.List(ctx, tenantID, entity.AffiliationExternal)
}
