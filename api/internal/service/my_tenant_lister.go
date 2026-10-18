package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type myTenantLister struct {
	tenants gateway.TenantGateway
	members gateway.MembershipGateway
	authz   usecase.Authorizer
}

func NewMyTenantLister(tenants gateway.TenantGateway, members gateway.MembershipGateway, authz usecase.Authorizer) usecase.MyTenantLister {
	return &myTenantLister{tenants: tenants, members: members, authz: authz}
}

// ListMine is ListObjects can_read_tenant, then a Postgres load of those ids
// with the membership labels.
func (s *myTenantLister) ListMine(ctx context.Context, actor string) ([]usecase.TenantView, error) {
	ids, err := s.authz.ListObjectIDs(ctx, actor, "can_read_tenant", entity.ObjectTypeTenant)
	if err != nil {
		return nil, err
	}
	out := []usecase.TenantView{}
	if len(ids) == 0 {
		return out, nil
	}
	tenants, err := s.tenants.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	memberships, err := s.members.ListForUser(ctx, actor)
	if err != nil {
		return nil, err
	}
	labels := map[string]entity.Membership{}
	for _, m := range memberships {
		labels[m.TenantID] = m
	}
	for _, t := range tenants {
		m := labels[t.ID]
		out = append(out, usecase.TenantView{Tenant: t, Affiliation: m.Affiliation, Role: m.Role})
	}
	return out, nil
}
