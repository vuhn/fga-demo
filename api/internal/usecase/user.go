package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type UserLister interface {
	List(ctx context.Context, actor string) ([]model.User, error)
}

type CreateUserInput struct {
	Email    string
	Name     string
	Password string
	// CompanyID is the user's employer. Required.
	CompanyID string
}

type UserCreator interface {
	Create(ctx context.Context, actor string, in CreateUserInput) (model.User, error)
}

type UserUpdater interface {
	UpdateName(ctx context.Context, actor, id, name string) (model.User, error)
}
