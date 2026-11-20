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

type memberLister struct {
	tenants gateway.TenantGateway
	members gateway.MembershipGateway
	authz   helper.Authorizer
}

func NewMemberLister(tenants gateway.TenantGateway, members gateway.MembershipGateway, authz helper.Authorizer) usecase.MemberLister {
	return &memberLister{tenants: tenants, members: members, authz: authz}
}

// List is tenant administration: only someone who can manage members (the
// internal admin) or the platform may read the roster. The affiliation
// filter is a display query after the Check.
func (s *memberLister) List(ctx context.Context, in input.MemberLister) (output.MemberLister, error) {
	switch in.Affiliation {
	case "", model.AffiliationInternal, model.AffiliationExternal:
	default:
		return output.MemberLister{}, model.Invalid("affiliation must be internal or external")
	}
	if err := s.authz.PlatformOr(ctx, in.Actor, "can_manage_members", in.TenantID, model.ErrForbidden); err != nil {
		return output.MemberLister{}, err
	}
	if _, err := s.tenants.Get(ctx, in.TenantID); err != nil {
		return output.MemberLister{}, err
	}
	got, err := s.members.List(ctx, in.TenantID, in.Affiliation)
	return output.MemberLister{Items: mapOutput(got, membershipOutput)}, err
}
