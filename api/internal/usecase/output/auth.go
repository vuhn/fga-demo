package output

import (
	"time"
)

// Authenticator is the result of Authenticator.Login.
type Authenticator struct {
	Token     string
	ExpiresAt time.Time
	User      User
}

// ProfileGetter is the result of ProfileGetter.Me: the caller and the tenants they can read.
type ProfileGetter struct {
	User    User
	Tenants []TenantView
}

// TokenVerifier is the result of TokenVerifier.Verify: the actor (the JWT sub).
type TokenVerifier struct {
	Actor string
}
