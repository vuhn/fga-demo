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

type myTenantLister struct {
	tenants gateway.TenantGateway
	members gateway.MembershipGateway
	authz   helper.Authorizer
}

func NewMyTenantLister(tenants gateway.TenantGateway, members gateway.MembershipGateway, authz helper.Authorizer) usecase.MyTenantLister {
	return &myTenantLister{tenants: tenants, members: members, authz: authz}
}

// ListMine is ListObjects can_read_tenant, then a Postgres load of those ids
// with the membership labels.
func (s *myTenantLister) ListMine(ctx context.Context, in input.MyTenantLister) (output.MyTenantLister, error) {
	ids, err := s.authz.ListObjectIDs(ctx, in.Actor, "can_read_tenant", model.ObjectTypeTenant)
	if err != nil {
		return output.MyTenantLister{}, err
	}
	out := []output.TenantView{}
	if len(ids) == 0 {
		return output.MyTenantLister{Items: out}, nil
	}
	tenants, err := s.tenants.ListByIDs(ctx, ids)
	if err != nil {
		return output.MyTenantLister{}, err
	}
	memberships, err := s.members.ListForUser(ctx, in.Actor)
	if err != nil {
		return output.MyTenantLister{}, err
	}
	labels := map[string]model.Membership{}
	for _, m := range memberships {
		labels[m.TenantID] = m
	}
	for _, t := range tenants {
		m := labels[t.ID]
		out = append(out, output.TenantView{Tenant: tenantOutput(t), Affiliation: m.Affiliation, Role: m.Role, RoleName: m.RoleName})
	}
	return output.MyTenantLister{Items: out}, nil
}
