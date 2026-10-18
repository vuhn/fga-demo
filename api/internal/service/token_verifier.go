package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type tokenVerifier struct {
	tokens gateway.TokenGateway
}

func NewTokenVerifier(tokens gateway.TokenGateway) usecase.TokenVerifier {
	return &tokenVerifier{tokens: tokens}
}

// Verify returns the token's subject. Any token problem is unauthenticated.
func (s *tokenVerifier) Verify(_ context.Context, token string) (string, error) {
	sub, err := s.tokens.Verify(token)
	if err != nil {
		return "", entity.ErrUnauthenticated
	}
	return sub, nil
}
