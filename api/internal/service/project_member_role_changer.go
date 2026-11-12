package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type projectMemberRoleChanger struct {
	projects  gateway.ProjectGateway
	members   gateway.ProjectMemberGateway
	roles     gateway.ProjectRoleGateway
	relations gateway.ProjectMemberRelationGateway
	roleRels  gateway.ProjectRoleRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewProjectMemberRoleChanger(projects gateway.ProjectGateway, members gateway.ProjectMemberGateway, roles gateway.ProjectRoleGateway,
	relations gateway.ProjectMemberRelationGateway, roleRels gateway.ProjectRoleRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.ProjectMemberRoleChanger {
	return &projectMemberRoleChanger{projects: projects, members: members, roles: roles, relations: relations, roleRels: roleRels, authz: authz, guard: guard}
}

// ChangeRole keeps the affiliation. A role from the other affiliation is a mismatch.
func (s *projectMemberRoleChanger) ChangeRole(ctx context.Context, actor, tenantID, projectID, userID, role string) (entity.ProjectMember, error) {
	if _, err := loadProject(ctx, s.projects, tenantID, projectID); err != nil {
		return entity.ProjectMember{}, err
	}
	if err := requireProjectManager(ctx, s.authz, actor, projectID); err != nil {
		return entity.ProjectMember{}, err
	}
	m, err := s.members.Get(ctx, projectID, userID)
	if err != nil {
		return entity.ProjectMember{}, err
	}
	if m.Role == role {
		return m, nil
	}
	assigned, err := assignProjectRole(ctx, s.roles, s.relations, s.roleRels, projectID, userID, m.Affiliation, role, m.ProjectRoleID)
	if err != nil {
		return entity.ProjectMember{}, err
	}
	if err := s.members.UpdateRole(ctx, projectID, userID, assigned.StoredRole, assigned.ProjectRoleID); err != nil {
		return entity.ProjectMember{}, err
	}
	err = s.guard.Grant(ctx, assigned.Writes, assigned.Deletes, func(ctx context.Context) error {
		return s.members.UpdateRole(ctx, projectID, userID, m.Role, m.ProjectRoleID)
	})
	if err != nil {
		return entity.ProjectMember{}, err
	}
	return s.members.Get(ctx, projectID, userID)
}
