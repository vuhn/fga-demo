package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type memberLister struct {
	tenants gateway.TenantGateway
	members gateway.MembershipGateway
	authz   usecase.Authorizer
}

func NewMemberLister(tenants gateway.TenantGateway, members gateway.MembershipGateway, authz usecase.Authorizer) usecase.MemberLister {
	return &memberLister{tenants: tenants, members: members, authz: authz}
}

// List is tenant administration: only someone who can manage members (the
// internal admin) or the platform may read the roster. The affiliation
// filter is a display query after the Check.
func (s *memberLister) List(ctx context.Context, actor, tenantID, affiliation string) ([]entity.Membership, error) {
	switch affiliation {
	case "", entity.AffiliationInternal, entity.AffiliationExternal:
	default:
		return nil, entity.Invalid("affiliation must be internal or external")
	}
	if err := s.authz.PlatformOr(ctx, actor, "can_manage_members", tenantID, entity.ErrForbidden); err != nil {
		return nil, err
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return nil, err
	}
	return s.members.List(ctx, tenantID, affiliation)
}
