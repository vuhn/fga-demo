package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type tokenVerifier struct {
	tokens gateway.TokenGateway
}

func NewTokenVerifier(tokens gateway.TokenGateway) usecase.TokenVerifier {
	return &tokenVerifier{tokens: tokens}
}

// Verify returns the token's subject. Any token problem is unauthenticated.
func (s *tokenVerifier) Verify(_ context.Context, in input.TokenVerifier) (output.TokenVerifier, error) {
	sub, err := s.tokens.Verify(in.Token)
	if err != nil {
		return output.TokenVerifier{}, model.ErrUnauthenticated
	}
	return output.TokenVerifier{Actor: sub}, nil
}
