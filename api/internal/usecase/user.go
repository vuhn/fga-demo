package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type UserLister interface {
	List(ctx context.Context, actor string) ([]entity.User, error)
}

type CreateUserInput struct {
	Email    string
	Name     string
	Password string
}

type UserCreator interface {
	Create(ctx context.Context, actor string, in CreateUserInput) (entity.User, error)
}

type UserUpdater interface {
	UpdateName(ctx context.Context, actor, id, name string) (entity.User, error)
}
