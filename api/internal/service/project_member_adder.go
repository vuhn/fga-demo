package service

import (
	"context"
	"errors"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type projectMemberAdder struct {
	projects  gateway.ProjectGateway
	tenantMem gateway.MembershipGateway
	members   gateway.ProjectMemberGateway
	roles     gateway.ProjectRoleGateway
	relations gateway.ProjectMemberRelationGateway
	roleRels  gateway.ProjectRoleRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewProjectMemberAdder(projects gateway.ProjectGateway, tenantMem gateway.MembershipGateway, members gateway.ProjectMemberGateway, roles gateway.ProjectRoleGateway,
	relations gateway.ProjectMemberRelationGateway, roleRels gateway.ProjectRoleRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.ProjectMemberAdder {
	return &projectMemberAdder{projects: projects, tenantMem: tenantMem, members: members, roles: roles, relations: relations, roleRels: roleRels, authz: authz, guard: guard}
}

// Add makes an existing tenant member a project member. Only tenant members
// qualify (the row also has a foreign key to tenant_members), and the role
// must match the member's affiliation.
func (s *projectMemberAdder) Add(ctx context.Context, actor, tenantID, projectID string, in usecase.AddProjectMemberInput) (entity.ProjectMember, error) {
	if err := entity.RequireUUID("user_id", in.UserID); err != nil {
		return entity.ProjectMember{}, err
	}
	if _, err := loadProject(ctx, s.projects, tenantID, projectID); err != nil {
		return entity.ProjectMember{}, err
	}
	if err := requireProjectManager(ctx, s.authz, actor, projectID); err != nil {
		return entity.ProjectMember{}, err
	}
	tm, err := s.tenantMem.Get(ctx, tenantID, in.UserID)
	if errors.Is(err, entity.ErrNotFound) {
		return entity.ProjectMember{}, entity.ErrNotAMember
	}
	if err != nil {
		return entity.ProjectMember{}, err
	}
	assigned, err := assignProjectRole(ctx, s.roles, s.relations, s.roleRels, projectID, in.UserID, tm.Affiliation, in.Role, "")
	if err != nil {
		return entity.ProjectMember{}, err
	}
	m := entity.ProjectMember{ProjectID: projectID, TenantID: tenantID, UserID: in.UserID, Role: assigned.StoredRole, ProjectRoleID: assigned.ProjectRoleID}
	if err := s.members.Insert(ctx, m); err != nil {
		return entity.ProjectMember{}, err
	}
	err = s.guard.Grant(ctx, assigned.Writes, assigned.Deletes, func(ctx context.Context) error {
		return s.members.Delete(ctx, projectID, in.UserID)
	})
	if err != nil {
		return entity.ProjectMember{}, err
	}
	return s.members.Get(ctx, projectID, in.UserID)
}
