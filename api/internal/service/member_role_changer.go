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

type memberRoleChanger struct {
	members   gateway.MembershipGateway
	roles     gateway.RoleGateway
	relations gateway.MembershipRelationGateway
	roleRels  gateway.RoleRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
}

func NewMemberRoleChanger(members gateway.MembershipGateway, roles gateway.RoleGateway, relations gateway.MembershipRelationGateway, roleRels gateway.RoleRelationGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.MemberRoleChanger {
	return &memberRoleChanger{members: members, roles: roles, relations: relations, roleRels: roleRels, authz: authz, guard: guard}
}

// ChangeRole keeps the affiliation. A role from the other affiliation is a
// mismatch, not a move.
func (s *memberRoleChanger) ChangeRole(ctx context.Context, in input.MemberRoleChanger) (output.MemberRoleChanger, error) {
	if err := s.authz.PlatformOr(ctx, in.Actor, "can_manage_members", in.TenantID, model.ErrForbidden); err != nil {
		return output.MemberRoleChanger{}, err
	}
	m, err := s.members.Get(ctx, in.TenantID, in.UserID)
	if err != nil {
		return output.MemberRoleChanger{}, err
	}
	if m.Role == in.Role {
		return output.MemberRoleChanger{Membership: membershipOutput(m)}, nil
	}
	assigned, err := assignMemberRole(ctx, s.roles, s.relations, s.roleRels, in.TenantID, in.UserID, m.Affiliation, in.Role, m.CustomRoleID)
	if err != nil {
		return output.MemberRoleChanger{}, err
	}
	if err := keepOneInternalAdmin(ctx, s.members, m); err != nil {
		return output.MemberRoleChanger{}, err
	}

	if err := s.members.UpdateRole(ctx, model.UpdateMemberRoleParams{TenantID: in.TenantID, UserID: in.UserID, Role: assigned.StoredRole, CustomRoleID: assigned.CustomRoleID}); err != nil {
		return output.MemberRoleChanger{}, err
	}
	err = s.guard.Grant(ctx, assigned.Writes, assigned.Deletes, func(ctx context.Context) error {
		return s.members.UpdateRole(ctx, model.UpdateMemberRoleParams{TenantID: in.TenantID, UserID: in.UserID, Role: m.Role, CustomRoleID: m.CustomRoleID})
	})
	if err != nil {
		return output.MemberRoleChanger{}, err
	}
	got, err := s.members.Get(ctx, in.TenantID, in.UserID)
	return output.MemberRoleChanger{Membership: membershipOutput(got)}, err
}
