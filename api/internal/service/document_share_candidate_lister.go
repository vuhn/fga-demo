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

type documentShareCandidateLister struct {
	documents gateway.DocumentGateway
	members   gateway.MembershipGateway
	authz     helper.Authorizer
}

func NewDocumentShareCandidateLister(documents gateway.DocumentGateway, members gateway.MembershipGateway, authz helper.Authorizer) usecase.DocumentShareCandidateLister {
	return &documentShareCandidateLister{documents: documents, members: members, authz: authz}
}

// ListShareCandidates lists the external members of the document's tenant. It
// needs can_share on the document, not member management, so owners can share
// without seeing the full roster.
func (s *documentShareCandidateLister) ListShareCandidates(ctx context.Context, in input.DocumentShareCandidateLister) (output.DocumentShareCandidateLister, error) {
	if err := requireDocumentShare(ctx, s.documents, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return output.DocumentShareCandidateLister{}, err
	}
	got, err := s.members.List(ctx, in.TenantID, model.AffiliationExternal)
	return output.DocumentShareCandidateLister{Items: mapOutput(got, membershipOutput)}, err
}
