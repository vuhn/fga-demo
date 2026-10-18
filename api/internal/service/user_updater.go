package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type userUpdater struct {
	users gateway.UserGateway
	authz usecase.Authorizer
}

func NewUserUpdater(users gateway.UserGateway, authz usecase.Authorizer) usecase.UserUpdater {
	return &userUpdater{users: users, authz: authz}
}

// UpdateName is platform only.
func (s *userUpdater) UpdateName(ctx context.Context, actor, id, name string) (entity.User, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return entity.User{}, err
	}
	name, err := entity.CleanName("name", name)
	if err != nil {
		return entity.User{}, err
	}
	return s.users.UpdateName(ctx, id, name)
}
