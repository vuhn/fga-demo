package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type documentShareCandidateLister struct {
	documents gateway.DocumentGateway
	members   gateway.MembershipGateway
	authz     usecase.Authorizer
}

func NewDocumentShareCandidateLister(documents gateway.DocumentGateway, members gateway.MembershipGateway, authz usecase.Authorizer) usecase.DocumentShareCandidateLister {
	return &documentShareCandidateLister{documents: documents, members: members, authz: authz}
}

// ListShareCandidates lists the external members of the document's tenant. It
// needs can_share on the document, not member management, so owners can share
// without seeing the full roster.
func (s *documentShareCandidateLister) ListShareCandidates(ctx context.Context, actor, tenantID, id string) ([]entity.Membership, error) {
	if err := requireDocumentShare(ctx, s.documents, s.authz, actor, tenantID, id); err != nil {
		return nil, err
	}
	return s.members.List(ctx, tenantID, entity.AffiliationExternal)
}
