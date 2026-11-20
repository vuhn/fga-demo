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

type userUpdater struct {
	users gateway.UserGateway
	authz helper.Authorizer
}

func NewUserUpdater(users gateway.UserGateway, authz helper.Authorizer) usecase.UserUpdater {
	return &userUpdater{users: users, authz: authz}
}

// UpdateName is platform only.
func (s *userUpdater) UpdateName(ctx context.Context, in input.UserUpdater) (output.UserUpdater, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.UserUpdater{}, err
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.UserUpdater{}, err
	}
	got, err := s.users.UpdateName(ctx, in.ID, name)
	return output.UserUpdater{User: userOutput(got)}, err
}
