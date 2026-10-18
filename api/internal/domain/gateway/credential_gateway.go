package gateway

import "time"

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

// TokenGateway issues and verifies access tokens that carry only the user id.
type TokenGateway interface {
	Issue(userID string) (token string, expiresAt time.Time, err error)
	Verify(token string) (userID string, err error)
}

// IDGenerator returns new UUID strings.
type IDGenerator interface {
	NewID() string
}
