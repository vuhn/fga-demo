package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// loadDocument returns the document when it exists in the header tenant.
func loadDocument(ctx context.Context, documents gateway.DocumentGateway, tenantID, id string) (entity.Document, error) {
	if err := entity.RequireUUID("id", id); err != nil {
		return entity.Document{}, err
	}
	d, err := documents.Get(ctx, id)
	if err != nil {
		return entity.Document{}, err
	}
	if d.TenantID != tenantID {
		return entity.Document{}, entity.ErrNotFound
	}
	return d, nil
}

// requireDocumentShare loads the document in the tenant and requires can_share.
func requireDocumentShare(ctx context.Context, documents gateway.DocumentGateway, authz usecase.Authorizer, actor, tenantID, id string) error {
	if _, err := loadDocument(ctx, documents, tenantID, id); err != nil {
		return err
	}
	return authz.Require(ctx, actor, "can_share", entity.ObjectTypeDocument, id)
}

func documentID(d entity.Document) string { return d.ID }
