package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type projectMemberRemover struct {
	projects  gateway.ProjectGateway
	members   gateway.ProjectMemberGateway
	relations gateway.ProjectMemberRelationGateway
	roleRels  gateway.ProjectRoleRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewProjectMemberRemover(projects gateway.ProjectGateway, members gateway.ProjectMemberGateway, relations gateway.ProjectMemberRelationGateway,
	roleRels gateway.ProjectRoleRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.ProjectMemberRemover {
	return &projectMemberRemover{projects: projects, members: members, relations: relations, roleRels: roleRels, authz: authz, guard: guard}
}

// Remove drops the person from the project only; their tenant membership and
// tenant role stay. Tuples go first, then the row; a failed row delete restores them.
func (s *projectMemberRemover) Remove(ctx context.Context, actor, tenantID, projectID, userID string) error {
	if _, err := loadProject(ctx, s.projects, tenantID, projectID); err != nil {
		return err
	}
	if err := requireProjectManager(ctx, s.authz, actor, projectID); err != nil {
		return err
	}
	m, err := s.members.Get(ctx, projectID, userID)
	if err != nil {
		return err
	}
	deletes, restore, err := projectMemberTuples(s.relations, s.roleRels, m)
	if err != nil {
		return err
	}
	return s.guard.Revoke(ctx, deletes, restore, func(ctx context.Context) error {
		return s.members.Delete(ctx, projectID, userID)
	})
}

// projectMemberTuples returns every tuple one project membership owns and the
// exact set that existed (for restoring).
func projectMemberTuples(relations gateway.ProjectMemberRelationGateway, roleRels gateway.ProjectRoleRelationGateway, m entity.ProjectMember) (deletes, restore []entity.Tuple, err error) {
	deletes = relations.ClearRoles(m.ProjectID, m.UserID)
	if m.ProjectRoleID != "" {
		a := roleRels.Assignee(m.ProjectRoleID, m.UserID)
		return append(deletes, a), []entity.Tuple{a}, nil
	}
	restore, _, err = relations.AssignRole(m.ProjectID, m.UserID, m.Affiliation, m.Role)
	return deletes, restore, err
}
