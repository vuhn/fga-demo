package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type documentDeleter struct {
	documents gateway.DocumentGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewDocumentDeleter(documents gateway.DocumentGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.DocumentDeleter {
	return &documentDeleter{documents: documents, authz: authz, guard: guard}
}

// Delete requires can_delete. It removes every tuple on the document, then
// the row and its grants.
func (s *documentDeleter) Delete(ctx context.Context, actor, tenantID, id string) error {
	if _, err := loadDocument(ctx, s.documents, tenantID, id); err != nil {
		return err
	}
	if err := s.authz.Require(ctx, actor, "can_delete", entity.ObjectTypeDocument, id); err != nil {
		return err
	}
	return s.guard.RevokeObject(ctx, entity.ObjectTypeDocument, id, func(ctx context.Context) error {
		return s.documents.Delete(ctx, id)
	})
}
