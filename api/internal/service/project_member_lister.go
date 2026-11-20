package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type projectMemberLister struct {
	projects gateway.ProjectGateway
	members  gateway.ProjectMemberGateway
	authz    helper.Authorizer
}

func NewProjectMemberLister(projects gateway.ProjectGateway, members gateway.ProjectMemberGateway, authz helper.Authorizer) usecase.ProjectMemberLister {
	return &projectMemberLister{projects: projects, members: members, authz: authz}
}

// List is project administration: project member managers or the platform.
func (s *projectMemberLister) List(ctx context.Context, in input.ProjectMemberLister) (output.ProjectMemberLister, error) {
	if _, err := loadProject(ctx, s.projects, in.TenantID, in.ProjectID); err != nil {
		return output.ProjectMemberLister{}, err
	}
	if err := requireProjectManager(ctx, s.authz, in.Actor, in.ProjectID); err != nil {
		return output.ProjectMemberLister{}, err
	}
	got, err := s.members.List(ctx, in.ProjectID)
	return output.ProjectMemberLister{Items: mapOutput(got, projectMemberOutput)}, err
}
