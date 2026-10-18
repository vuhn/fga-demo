// Package auth issues and verifies HS256 JWTs and hashes passwords.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

// JWT carries only the user id in `sub`. The tenant is a request header and
// roles live in OpenFGA, so neither is a claim.
type JWT struct {
	Secret []byte
	TTL    time.Duration
}

var _ gateway.TokenGateway = JWT{}

func (j JWT) Issue(userID string) (string, time.Time, error) {
	exp := time.Now().Add(j.TTL).UTC().Truncate(time.Second)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(exp),
	})
	s, err := tok.SignedString(j.Secret)
	return s, exp, err
}

func (j JWT) Verify(token string) (string, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return j.Secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return "", err
	}
	if claims.Subject == "" {
		return "", errors.New("token has no subject")
	}
	return claims.Subject, nil
}
