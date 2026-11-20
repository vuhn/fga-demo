package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type userLister struct {
	users gateway.UserGateway
	authz helper.Authorizer
}

func NewUserLister(users gateway.UserGateway, authz helper.Authorizer) usecase.UserLister {
	return &userLister{users: users, authz: authz}
}

// List is platform only.
func (s *userLister) List(ctx context.Context, in input.UserLister) (output.UserLister, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.UserLister{}, err
	}
	got, err := s.users.List(ctx)
	return output.UserLister{Items: mapOutput(got, userOutput)}, err
}
