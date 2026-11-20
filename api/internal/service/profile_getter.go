package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type profileGetter struct {
	users   gateway.UserGateway
	authz   helper.Authorizer
	tenants usecase.MyTenantLister
}

func NewProfileGetter(users gateway.UserGateway, authz helper.Authorizer, tenants usecase.MyTenantLister) usecase.ProfileGetter {
	return &profileGetter{users: users, authz: authz, tenants: tenants}
}

func (s *profileGetter) Me(ctx context.Context, in input.ProfileGetter) (output.ProfileGetter, error) {
	u, err := s.users.Get(ctx, in.Actor)
	if err != nil {
		return output.ProfileGetter{}, err
	}
	if u, err = withSystemAdmin(ctx, s.authz, u); err != nil {
		return output.ProfileGetter{}, err
	}
	tenants, err := s.tenants.ListMine(ctx, input.MyTenantLister{Actor: in.Actor})
	return output.ProfileGetter{User: userOutput(u), Tenants: tenants.Items}, err
}
