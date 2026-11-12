package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type projectMemberCandidateLister struct {
	projects gateway.ProjectGateway
	tenant   gateway.MembershipGateway
	members  gateway.ProjectMemberGateway
	authz    usecase.Authorizer
}

func NewProjectMemberCandidateLister(projects gateway.ProjectGateway, tenant gateway.MembershipGateway, members gateway.ProjectMemberGateway, authz usecase.Authorizer) usecase.ProjectMemberCandidateLister {
	return &projectMemberCandidateLister{projects: projects, tenant: tenant, members: members, authz: authz}
}

// ListCandidates needs the project member manager check; the filter that drops
// current project members is display only.
func (s *projectMemberCandidateLister) ListCandidates(ctx context.Context, actor, tenantID, projectID string) ([]entity.Membership, error) {
	if _, err := loadProject(ctx, s.projects, tenantID, projectID); err != nil {
		return nil, err
	}
	if err := requireProjectManager(ctx, s.authz, actor, projectID); err != nil {
		return nil, err
	}
	current, err := s.members.List(ctx, projectID)
	if err != nil {
		return nil, err
	}
	in := map[string]bool{}
	for _, m := range current {
		in[m.UserID] = true
	}
	all, err := s.tenant.List(ctx, tenantID, "")
	if err != nil {
		return nil, err
	}
	out := make([]entity.Membership, 0, len(all))
	for _, m := range all {
		if !in[m.UserID] {
			out = append(out, m)
		}
	}
	return out, nil
}
