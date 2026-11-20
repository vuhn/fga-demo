package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type Authenticator interface {
	Login(ctx context.Context, in input.Authenticator) (output.Authenticator, error)
}

// TokenVerifier resolves a bearer token to the actor (the JWT sub).
type TokenVerifier interface {
	Verify(ctx context.Context, in input.TokenVerifier) (output.TokenVerifier, error)
}

// ProfileGetter returns the caller and the tenants they can read.
type ProfileGetter interface {
	Me(ctx context.Context, in input.ProfileGetter) (output.ProfileGetter, error)
}
