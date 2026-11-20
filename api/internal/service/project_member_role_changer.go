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

type projectMemberRoleChanger struct {
	projects  gateway.ProjectGateway
	members   gateway.ProjectMemberGateway
	roles     gateway.ProjectRoleGateway
	relations gateway.ProjectMemberRelationGateway
	roleRels  gateway.ProjectRoleRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
}

func NewProjectMemberRoleChanger(projects gateway.ProjectGateway, members gateway.ProjectMemberGateway, roles gateway.ProjectRoleGateway,
	relations gateway.ProjectMemberRelationGateway, roleRels gateway.ProjectRoleRelationGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.ProjectMemberRoleChanger {
	return &projectMemberRoleChanger{projects: projects, members: members, roles: roles, relations: relations, roleRels: roleRels, authz: authz, guard: guard}
}

// ChangeRole keeps the affiliation. A role from the other affiliation is a mismatch.
func (s *projectMemberRoleChanger) ChangeRole(ctx context.Context, in input.ProjectMemberRoleChanger) (output.ProjectMemberRoleChanger, error) {
	if _, err := loadProject(ctx, s.projects, in.TenantID, in.ProjectID); err != nil {
		return output.ProjectMemberRoleChanger{}, err
	}
	if err := requireProjectManager(ctx, s.authz, in.Actor, in.ProjectID); err != nil {
		return output.ProjectMemberRoleChanger{}, err
	}
	m, err := s.members.Get(ctx, in.ProjectID, in.UserID)
	if err != nil {
		return output.ProjectMemberRoleChanger{}, err
	}
	if m.Role == in.Role {
		return output.ProjectMemberRoleChanger{ProjectMember: projectMemberOutput(m)}, nil
	}
	assigned, err := assignProjectRole(ctx, s.roles, s.relations, s.roleRels, in.ProjectID, in.UserID, m.Affiliation, in.Role, m.ProjectRoleID)
	if err != nil {
		return output.ProjectMemberRoleChanger{}, err
	}
	if err := s.members.UpdateRole(ctx, model.UpdateProjectMemberRoleParams{ProjectID: in.ProjectID, UserID: in.UserID, Role: assigned.StoredRole, ProjectRoleID: assigned.ProjectRoleID}); err != nil {
		return output.ProjectMemberRoleChanger{}, err
	}
	err = s.guard.Grant(ctx, assigned.Writes, assigned.Deletes, func(ctx context.Context) error {
		return s.members.UpdateRole(ctx, model.UpdateProjectMemberRoleParams{ProjectID: in.ProjectID, UserID: in.UserID, Role: m.Role, ProjectRoleID: m.ProjectRoleID})
	})
	if err != nil {
		return output.ProjectMemberRoleChanger{}, err
	}
	got, err := s.members.Get(ctx, in.ProjectID, in.UserID)
	return output.ProjectMemberRoleChanger{ProjectMember: projectMemberOutput(got)}, err
}
