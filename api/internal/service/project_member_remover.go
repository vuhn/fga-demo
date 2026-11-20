package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type projectMemberRemover struct {
	projects  gateway.ProjectGateway
	members   gateway.ProjectMemberGateway
	relations gateway.ProjectMemberRelationGateway
	roleRels  gateway.ProjectRoleRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
}

func NewProjectMemberRemover(projects gateway.ProjectGateway, members gateway.ProjectMemberGateway, relations gateway.ProjectMemberRelationGateway,
	roleRels gateway.ProjectRoleRelationGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.ProjectMemberRemover {
	return &projectMemberRemover{projects: projects, members: members, relations: relations, roleRels: roleRels, authz: authz, guard: guard}
}

// Remove drops the person from the project only; their tenant membership and
// tenant role stay. Tuples go first, then the row; a failed row delete restores them.
func (s *projectMemberRemover) Remove(ctx context.Context, in input.ProjectMemberRemover) error {
	if _, err := loadProject(ctx, s.projects, in.TenantID, in.ProjectID); err != nil {
		return err
	}
	if err := requireProjectManager(ctx, s.authz, in.Actor, in.ProjectID); err != nil {
		return err
	}
	m, err := s.members.Get(ctx, in.ProjectID, in.UserID)
	if err != nil {
		return err
	}
	deletes, restore, err := projectMemberTuples(s.relations, s.roleRels, m)
	if err != nil {
		return err
	}
	return s.guard.Revoke(ctx, deletes, restore, func(ctx context.Context) error {
		return s.members.Delete(ctx, in.ProjectID, in.UserID)
	})
}

// projectMemberTuples returns every tuple one project membership owns and the
// exact set that existed (for restoring).
func projectMemberTuples(relations gateway.ProjectMemberRelationGateway, roleRels gateway.ProjectRoleRelationGateway, m model.ProjectMember) (deletes, restore []model.Tuple, err error) {
	deletes = relations.ClearRoles(m.ProjectID, m.UserID)
	if m.ProjectRoleID != "" {
		a := roleRels.Assignee(m.ProjectRoleID, m.UserID)
		return append(deletes, a), []model.Tuple{a}, nil
	}
	restore, _, err = relations.AssignRole(model.AssignProjectMemberRoleParams{ProjectID: m.ProjectID, UserID: m.UserID, Affiliation: m.Affiliation, Role: m.Role})
	return deletes, restore, err
}
