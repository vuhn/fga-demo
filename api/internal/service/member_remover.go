package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type memberRemover struct {
	members         gateway.MembershipGateway
	relations       gateway.MembershipRelationGateway
	roles           gateway.RoleGateway
	roleRels        gateway.RoleRelationGateway
	projectMembers  gateway.ProjectMemberGateway
	projectRels     gateway.ProjectMemberRelationGateway
	projectRoleRels gateway.ProjectRoleRelationGateway
	authz           helper.Authorizer
	guard           helper.WriteGuard
}

func NewMemberRemover(members gateway.MembershipGateway, relations gateway.MembershipRelationGateway, roles gateway.RoleGateway, roleRels gateway.RoleRelationGateway, projectMembers gateway.ProjectMemberGateway, projectRels gateway.ProjectMemberRelationGateway, projectRoleRels gateway.ProjectRoleRelationGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.MemberRemover {
	return &memberRemover{members: members, relations: relations, roles: roles, roleRels: roleRels, projectMembers: projectMembers, projectRels: projectRels, projectRoleRels: projectRoleRels, authz: authz, guard: guard}
}

// Remove deletes the built-in role tuples, custom-role assignee tuples,
// project membership tuples, and every direct grant for the user in this tenant. Group usersets stay; the
// person simply drops out of them.
func (s *memberRemover) Remove(ctx context.Context, in input.MemberRemover) error {
	if err := s.authz.PlatformOr(ctx, in.Actor, "can_manage_members", in.TenantID, model.ErrForbidden); err != nil {
		return err
	}
	m, err := s.members.Get(ctx, in.TenantID, in.UserID)
	if err != nil {
		return err
	}
	if err := keepOneInternalAdmin(ctx, s.members, m); err != nil {
		return err
	}
	grants, err := s.members.ListUserGrants(ctx, in.TenantID, in.UserID)
	if err != nil {
		return err
	}
	deletes := s.relations.Removal(in.TenantID, in.UserID, grants)
	roleIDs, err := s.roles.ListRoleIDsForUser(ctx, in.TenantID, in.UserID)
	if err != nil {
		return err
	}
	for _, id := range roleIDs {
		deletes = append(deletes, s.roleRels.Assignee(id, in.UserID))
	}

	// Restore set: exactly the tuples that existed.
	var restore []model.Tuple
	if m.CustomRoleID == "" {
		var err error
		restore, _, err = s.relations.AssignRole(model.AssignMemberRoleParams{TenantID: in.TenantID, UserID: in.UserID, Affiliation: m.Affiliation, Role: m.Role})
		if err != nil {
			return err
		}
	}
	grantTuples, err := s.relations.Grants(grants)
	if err != nil {
		return err
	}
	restore = append(restore, grantTuples...)
	for _, id := range roleIDs {
		restore = append(restore, s.roleRels.Assignee(id, in.UserID))
	}

	// Project memberships go with the tenant membership (cascade in Postgres),
	// so their tuples are deleted here too.
	pms, err := s.projectMembers.ListForUser(ctx, in.TenantID, in.UserID)
	if err != nil {
		return err
	}
	for _, pm := range pms {
		d, r, err := projectMemberTuples(s.projectRels, s.projectRoleRels, pm)
		if err != nil {
			return err
		}
		deletes = append(deletes, d...)
		restore = append(restore, r...)
	}

	return s.guard.Revoke(ctx, deletes, restore, func(ctx context.Context) error {
		return s.members.Delete(ctx, in.TenantID, in.UserID)
	})
}
