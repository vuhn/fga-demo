package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type TaskCreator interface {
	Create(ctx context.Context, in input.TaskCreator) (output.TaskCreator, error)
}
