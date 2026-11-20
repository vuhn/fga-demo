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

type projectMemberCandidateLister struct {
	projects gateway.ProjectGateway
	tenant   gateway.MembershipGateway
	members  gateway.ProjectMemberGateway
	authz    helper.Authorizer
}

func NewProjectMemberCandidateLister(projects gateway.ProjectGateway, tenant gateway.MembershipGateway, members gateway.ProjectMemberGateway, authz helper.Authorizer) usecase.ProjectMemberCandidateLister {
	return &projectMemberCandidateLister{projects: projects, tenant: tenant, members: members, authz: authz}
}

// ListCandidates needs the project member manager check; the filter that drops
// current project members is display only.
func (s *projectMemberCandidateLister) ListCandidates(ctx context.Context, in input.ProjectMemberCandidateLister) (output.ProjectMemberCandidateLister, error) {
	if _, err := loadProject(ctx, s.projects, in.TenantID, in.ProjectID); err != nil {
		return output.ProjectMemberCandidateLister{}, err
	}
	if err := requireProjectManager(ctx, s.authz, in.Actor, in.ProjectID); err != nil {
		return output.ProjectMemberCandidateLister{}, err
	}
	current, err := s.members.List(ctx, in.ProjectID)
	if err != nil {
		return output.ProjectMemberCandidateLister{}, err
	}
	members := map[string]bool{}
	for _, m := range current {
		members[m.UserID] = true
	}
	all, err := s.tenant.List(ctx, in.TenantID, "")
	if err != nil {
		return output.ProjectMemberCandidateLister{}, err
	}
	out := make([]model.Membership, 0, len(all))
	for _, m := range all {
		if !members[m.UserID] {
			out = append(out, m)
		}
	}
	return output.ProjectMemberCandidateLister{Items: mapOutput(out, membershipOutput)}, nil
}
