package service

import (
	"context"
	"errors"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type authenticator struct {
	users  gateway.UserGateway
	hasher gateway.PasswordHasher
	tokens gateway.TokenGateway
	authz  helper.Authorizer
}

func NewAuthenticator(users gateway.UserGateway, hasher gateway.PasswordHasher, tokens gateway.TokenGateway, authz helper.Authorizer) usecase.Authenticator {
	return &authenticator{users: users, hasher: hasher, tokens: tokens, authz: authz}
}

// Login checks the password and issues a token whose only claim is the user id.
func (s *authenticator) Login(ctx context.Context, in input.Authenticator) (output.Authenticator, error) {
	u, err := s.users.GetByEmail(ctx, in.Email)
	if errors.Is(err, model.ErrNotFound) {
		return output.Authenticator{}, model.ErrUnauthenticated
	}
	if err != nil {
		return output.Authenticator{}, err
	}
	if !s.hasher.Compare(u.PasswordHash, in.Password) {
		return output.Authenticator{}, model.ErrUnauthenticated
	}
	if u, err = withSystemAdmin(ctx, s.authz, u); err != nil {
		return output.Authenticator{}, err
	}
	tok, exp, err := s.tokens.Issue(u.ID)
	if err != nil {
		return output.Authenticator{}, err
	}
	return output.Authenticator{Token: tok, ExpiresAt: exp, User: userOutput(u)}, nil
}

// withSystemAdmin sets IsSystemAdmin from the OpenFGA Check, not the table.
func withSystemAdmin(ctx context.Context, authz helper.Authorizer, u model.User) (model.User, error) {
	ok, err := authz.IsPlatformAdmin(ctx, u.ID)
	u.IsSystemAdmin = ok
	return u, err
}
