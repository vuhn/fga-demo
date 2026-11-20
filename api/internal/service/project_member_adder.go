package service

import (
	"context"
	"errors"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type projectMemberAdder struct {
	projects  gateway.ProjectGateway
	tenantMem gateway.MembershipGateway
	members   gateway.ProjectMemberGateway
	roles     gateway.ProjectRoleGateway
	relations gateway.ProjectMemberRelationGateway
	roleRels  gateway.ProjectRoleRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
}

func NewProjectMemberAdder(projects gateway.ProjectGateway, tenantMem gateway.MembershipGateway, members gateway.ProjectMemberGateway, roles gateway.ProjectRoleGateway,
	relations gateway.ProjectMemberRelationGateway, roleRels gateway.ProjectRoleRelationGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.ProjectMemberAdder {
	return &projectMemberAdder{projects: projects, tenantMem: tenantMem, members: members, roles: roles, relations: relations, roleRels: roleRels, authz: authz, guard: guard}
}

// Add makes an existing tenant member a project member. Only tenant members
// qualify (the row also has a foreign key to tenant_members), and the role
// must match the member's affiliation.
func (s *projectMemberAdder) Add(ctx context.Context, in input.ProjectMemberAdder) (output.ProjectMemberAdder, error) {
	if err := model.RequireUUID("user_id", in.UserID); err != nil {
		return output.ProjectMemberAdder{}, err
	}
	if _, err := loadProject(ctx, s.projects, in.TenantID, in.ProjectID); err != nil {
		return output.ProjectMemberAdder{}, err
	}
	if err := requireProjectManager(ctx, s.authz, in.Actor, in.ProjectID); err != nil {
		return output.ProjectMemberAdder{}, err
	}
	tm, err := s.tenantMem.Get(ctx, in.TenantID, in.UserID)
	if errors.Is(err, model.ErrNotFound) {
		return output.ProjectMemberAdder{}, model.ErrNotAMember
	}
	if err != nil {
		return output.ProjectMemberAdder{}, err
	}
	assigned, err := assignProjectRole(ctx, s.roles, s.relations, s.roleRels, in.ProjectID, in.UserID, tm.Affiliation, in.Role, "")
	if err != nil {
		return output.ProjectMemberAdder{}, err
	}
	m := model.ProjectMember{ProjectID: in.ProjectID, TenantID: in.TenantID, UserID: in.UserID, Role: assigned.StoredRole, ProjectRoleID: assigned.ProjectRoleID}
	if err := s.members.Insert(ctx, m); err != nil {
		return output.ProjectMemberAdder{}, err
	}
	err = s.guard.Grant(ctx, assigned.Writes, assigned.Deletes, func(ctx context.Context) error {
		return s.members.Delete(ctx, in.ProjectID, in.UserID)
	})
	if err != nil {
		return output.ProjectMemberAdder{}, err
	}
	got, err := s.members.Get(ctx, in.ProjectID, in.UserID)
	return output.ProjectMemberAdder{ProjectMember: projectMemberOutput(got)}, err
}
