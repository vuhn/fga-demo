package usecase

import (
	"context"
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	User      entity.User
}

type Authenticator interface {
	Login(ctx context.Context, email, password string) (LoginResult, error)
}

// TokenVerifier resolves a bearer token to the actor (the JWT sub).
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (actor string, err error)
}

// ProfileGetter returns the caller and the tenants they can read.
type ProfileGetter interface {
	Me(ctx context.Context, actor string) (entity.User, []TenantView, error)
}
