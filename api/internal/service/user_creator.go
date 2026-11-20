package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type userCreator struct {
	users     gateway.UserGateway
	hasher    gateway.PasswordHasher
	authz     helper.Authorizer
	ids       gateway.IDGenerator
	companies gateway.CompanyGateway
}

func NewUserCreator(users gateway.UserGateway, hasher gateway.PasswordHasher, authz helper.Authorizer, ids gateway.IDGenerator, companies gateway.CompanyGateway) usecase.UserCreator {
	return &userCreator{users: users, hasher: hasher, authz: authz, ids: ids, companies: companies}
}

// Create is platform only. A new user belongs to one company and to no tenant.
func (s *userCreator) Create(ctx context.Context, in input.UserCreator) (output.UserCreator, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.UserCreator{}, err
	}
	email := strings.TrimSpace(in.Email)
	if !strings.Contains(email, "@") {
		return output.UserCreator{}, model.Invalid("email must contain @")
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.UserCreator{}, err
	}
	if n := len(in.Password); n < 8 || n > 72 {
		return output.UserCreator{}, model.Invalid("password length must be 8 to 72")
	}
	if err := model.RequireUUID("company_id", in.CompanyID); err != nil {
		return output.UserCreator{}, err
	}
	if _, err := s.companies.Get(ctx, in.CompanyID); err != nil {
		return output.UserCreator{}, err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return output.UserCreator{}, err
	}
	u := model.User{ID: s.ids.NewID(), Email: email, Name: name, PasswordHash: hash, CompanyID: in.CompanyID}
	if err := s.users.Create(ctx, u); err != nil {
		return output.UserCreator{}, err
	}
	got, err := s.users.Get(ctx, u.ID)
	return output.UserCreator{User: userOutput(got)}, err
}
