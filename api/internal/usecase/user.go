package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type UserLister interface {
	List(ctx context.Context, in input.UserLister) (output.UserLister, error)
}

type UserCreator interface {
	Create(ctx context.Context, in input.UserCreator) (output.UserCreator, error)
}

type UserUpdater interface {
	UpdateName(ctx context.Context, in input.UserUpdater) (output.UserUpdater, error)
}
