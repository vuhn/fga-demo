package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type userLister struct {
	users gateway.UserGateway
	authz usecase.Authorizer
}

func NewUserLister(users gateway.UserGateway, authz usecase.Authorizer) usecase.UserLister {
	return &userLister{users: users, authz: authz}
}

// List is platform only.
func (s *userLister) List(ctx context.Context, actor string) ([]entity.User, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return nil, err
	}
	return s.users.List(ctx)
}
