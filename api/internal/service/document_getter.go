package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type documentGetter struct {
	documents gateway.DocumentGateway
	authz     usecase.Authorizer
}

func NewDocumentGetter(documents gateway.DocumentGateway, authz usecase.Authorizer) usecase.DocumentGetter {
	return &documentGetter{documents: documents, authz: authz}
}

// Get is 404 when the caller cannot view the document.
func (s *documentGetter) Get(ctx context.Context, actor, tenantID, id string) (entity.Visible[entity.Document], error) {
	d, err := loadDocument(ctx, s.documents, tenantID, id)
	if err != nil {
		return entity.Visible[entity.Document]{}, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_view", entity.ObjectTypeDocument, id); err != nil {
		return entity.Visible[entity.Document]{}, err
	}
	vs, err := withPerms(ctx, s.authz, actor, entity.ObjectTypeDocument, []entity.Document{d}, documentID)
	if err != nil {
		return entity.Visible[entity.Document]{}, err
	}
	return vs[0], nil
}
