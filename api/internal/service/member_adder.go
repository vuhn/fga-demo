package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type memberAdder struct {
	tenants   gateway.TenantGateway
	users     gateway.UserGateway
	members   gateway.MembershipGateway
	relations gateway.MembershipRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewMemberAdder(tenants gateway.TenantGateway, users gateway.UserGateway, members gateway.MembershipGateway,
	relations gateway.MembershipRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.MemberAdder {
	return &memberAdder{tenants: tenants, users: users, members: members, relations: relations, authz: authz, guard: guard}
}

func (s *memberAdder) Add(ctx context.Context, actor, tenantID string, in usecase.AddMemberInput) (entity.Membership, error) {
	if err := entity.RequireUUID("user_id", in.UserID); err != nil {
		return entity.Membership{}, err
	}
	if err := s.authz.PlatformOr(ctx, actor, "can_manage_members", tenantID, entity.ErrForbidden); err != nil {
		return entity.Membership{}, err
	}
	// Validates the affiliation/role pair before anything is written.
	writes, deletes, err := s.relations.AssignRole(tenantID, in.UserID, in.Affiliation, in.Role)
	if err != nil {
		return entity.Membership{}, err
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return entity.Membership{}, err
	}
	if _, err := s.users.Get(ctx, in.UserID); err != nil {
		return entity.Membership{}, err
	}

	m := entity.Membership{TenantID: tenantID, UserID: in.UserID, Affiliation: in.Affiliation, Role: in.Role}
	if err := s.members.Insert(ctx, m); err != nil {
		return entity.Membership{}, err
	}
	err = s.guard.Grant(ctx, writes, deletes, func(ctx context.Context) error {
		return s.members.Delete(ctx, tenantID, in.UserID)
	})
	if err != nil {
		return entity.Membership{}, err
	}
	return s.members.Get(ctx, tenantID, in.UserID)
}
