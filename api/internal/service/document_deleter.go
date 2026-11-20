package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type documentDeleter struct {
	documents gateway.DocumentGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
}

func NewDocumentDeleter(documents gateway.DocumentGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.DocumentDeleter {
	return &documentDeleter{documents: documents, authz: authz, guard: guard}
}

// Delete requires can_delete. It removes every tuple on the document, then
// the row and its grants.
func (s *documentDeleter) Delete(ctx context.Context, in input.DocumentDeleter) error {
	if _, err := loadDocument(ctx, s.documents, in.TenantID, in.ID); err != nil {
		return err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_delete", model.ObjectTypeDocument, in.ID); err != nil {
		return err
	}
	return s.guard.RevokeObject(ctx, model.ObjectTypeDocument, in.ID, func(ctx context.Context) error {
		return s.documents.Delete(ctx, in.ID)
	})
}
