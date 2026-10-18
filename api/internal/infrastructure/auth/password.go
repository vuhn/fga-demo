package auth

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type Bcrypt struct{}

var _ gateway.PasswordHasher = Bcrypt{}

func (Bcrypt) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func (Bcrypt) Compare(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
