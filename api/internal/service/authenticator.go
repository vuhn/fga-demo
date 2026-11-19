package service

import (
	"context"
	"errors"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type authenticator struct {
	users  gateway.UserGateway
	hasher gateway.PasswordHasher
	tokens gateway.TokenGateway
	authz  usecase.Authorizer
}

func NewAuthenticator(users gateway.UserGateway, hasher gateway.PasswordHasher, tokens gateway.TokenGateway, authz usecase.Authorizer) usecase.Authenticator {
	return &authenticator{users: users, hasher: hasher, tokens: tokens, authz: authz}
}

// Login checks the password and issues a token whose only claim is the user id.
func (s *authenticator) Login(ctx context.Context, email, password string) (usecase.LoginResult, error) {
	u, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, model.ErrNotFound) {
		return usecase.LoginResult{}, model.ErrUnauthenticated
	}
	if err != nil {
		return usecase.LoginResult{}, err
	}
	if !s.hasher.Compare(u.PasswordHash, password) {
		return usecase.LoginResult{}, model.ErrUnauthenticated
	}
	if u, err = withSystemAdmin(ctx, s.authz, u); err != nil {
		return usecase.LoginResult{}, err
	}
	tok, exp, err := s.tokens.Issue(u.ID)
	if err != nil {
		return usecase.LoginResult{}, err
	}
	return usecase.LoginResult{Token: tok, ExpiresAt: exp, User: u}, nil
}

// withSystemAdmin sets IsSystemAdmin from the OpenFGA Check, not the table.
func withSystemAdmin(ctx context.Context, authz usecase.Authorizer, u model.User) (model.User, error) {
	ok, err := authz.IsPlatformAdmin(ctx, u.ID)
	u.IsSystemAdmin = ok
	return u, err
}
