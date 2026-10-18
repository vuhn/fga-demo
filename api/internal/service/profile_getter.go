package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type profileGetter struct {
	users   gateway.UserGateway
	authz   usecase.Authorizer
	tenants usecase.MyTenantLister
}

func NewProfileGetter(users gateway.UserGateway, authz usecase.Authorizer, tenants usecase.MyTenantLister) usecase.ProfileGetter {
	return &profileGetter{users: users, authz: authz, tenants: tenants}
}

func (s *profileGetter) Me(ctx context.Context, actor string) (entity.User, []usecase.TenantView, error) {
	u, err := s.users.Get(ctx, actor)
	if err != nil {
		return entity.User{}, nil, err
	}
	if u, err = withSystemAdmin(ctx, s.authz, u); err != nil {
		return entity.User{}, nil, err
	}
	tenants, err := s.tenants.ListMine(ctx, actor)
	return u, tenants, err
}
