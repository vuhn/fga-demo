package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type documentRenamer struct {
	documents gateway.DocumentGateway
	authz     usecase.Authorizer
}

func NewDocumentRenamer(documents gateway.DocumentGateway, authz usecase.Authorizer) usecase.DocumentRenamer {
	return &documentRenamer{documents: documents, authz: authz}
}

// Rename requires can_edit.
func (s *documentRenamer) Rename(ctx context.Context, actor, tenantID, id, name string) (model.Document, error) {
	if _, err := loadDocument(ctx, s.documents, tenantID, id); err != nil {
		return model.Document{}, err
	}
	name, err := model.CleanName("name", name)
	if err != nil {
		return model.Document{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_edit", model.ObjectTypeDocument, id); err != nil {
		return model.Document{}, err
	}
	return s.documents.Rename(ctx, id, name)
}
