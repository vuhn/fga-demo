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

type projectLister struct {
	projects gateway.ProjectGateway
	authz    helper.Authorizer
}

func NewProjectLister(projects gateway.ProjectGateway, authz helper.Authorizer) usecase.ProjectLister {
	return &projectLister{projects: projects, authz: authz}
}

// List is ListObjects can_view, then a Postgres load of those ids in the tenant.
func (s *projectLister) List(ctx context.Context, in input.ProjectLister) (output.ProjectLister, error) {
	ids, err := s.authz.ListObjectIDs(ctx, in.Actor, "can_view", model.ObjectTypeProject)
	if err != nil || len(ids) == 0 {
		return output.ProjectLister{}, err
	}
	ps, err := s.projects.ListByIDs(ctx, in.TenantID, ids)
	if err != nil {
		return output.ProjectLister{}, err
	}
	got, err := withPerms(ctx, s.authz, in.Actor, model.ObjectTypeProject, ps, projectID)
	return output.ProjectLister{Items: mapVisibleOutput(got, projectOutput)}, err
}
