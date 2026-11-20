package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

// PermissionReader answers "what may I do here" for the caller only.
type PermissionReader interface {
	Mine(ctx context.Context, in input.PermissionReader) (output.PermissionReader, error)
}
