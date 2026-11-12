package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type projectMemberLister struct {
	projects gateway.ProjectGateway
	members  gateway.ProjectMemberGateway
	authz    usecase.Authorizer
}

func NewProjectMemberLister(projects gateway.ProjectGateway, members gateway.ProjectMemberGateway, authz usecase.Authorizer) usecase.ProjectMemberLister {
	return &projectMemberLister{projects: projects, members: members, authz: authz}
}

// List is project administration: project member managers or the platform.
func (s *projectMemberLister) List(ctx context.Context, actor, tenantID, projectID string) ([]entity.ProjectMember, error) {
	if _, err := loadProject(ctx, s.projects, tenantID, projectID); err != nil {
		return nil, err
	}
	if err := requireProjectManager(ctx, s.authz, actor, projectID); err != nil {
		return nil, err
	}
	return s.members.List(ctx, projectID)
}
