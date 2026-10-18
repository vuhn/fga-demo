package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type userCreator struct {
	users  gateway.UserGateway
	hasher gateway.PasswordHasher
	authz  usecase.Authorizer
	ids    gateway.IDGenerator
}

func NewUserCreator(users gateway.UserGateway, hasher gateway.PasswordHasher, authz usecase.Authorizer, ids gateway.IDGenerator) usecase.UserCreator {
	return &userCreator{users: users, hasher: hasher, authz: authz, ids: ids}
}

// Create is platform only. New users belong to no tenant.
func (s *userCreator) Create(ctx context.Context, actor string, in usecase.CreateUserInput) (entity.User, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return entity.User{}, err
	}
	email := strings.TrimSpace(in.Email)
	if !strings.Contains(email, "@") {
		return entity.User{}, entity.Invalid("email must contain @")
	}
	name, err := entity.CleanName("name", in.Name)
	if err != nil {
		return entity.User{}, err
	}
	if n := len(in.Password); n < 8 || n > 72 {
		return entity.User{}, entity.Invalid("password length must be 8 to 72")
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return entity.User{}, err
	}
	u := entity.User{ID: s.ids.NewID(), Email: email, Name: name, PasswordHash: hash}
	if err := s.users.Create(ctx, u); err != nil {
		return entity.User{}, err
	}
	return s.users.Get(ctx, u.ID)
}
