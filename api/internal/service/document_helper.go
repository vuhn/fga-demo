package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

// loadDocument returns the document when it exists in the header tenant.
func loadDocument(ctx context.Context, documents gateway.DocumentGateway, tenantID, id string) (model.Document, error) {
	if err := model.RequireUUID("id", id); err != nil {
		return model.Document{}, err
	}
	d, err := documents.Get(ctx, id)
	if err != nil {
		return model.Document{}, err
	}
	if d.TenantID != tenantID {
		return model.Document{}, model.ErrNotFound
	}
	return d, nil
}

// requireDocumentShare loads the document in the tenant and requires can_share.
func requireDocumentShare(ctx context.Context, documents gateway.DocumentGateway, authz helper.Authorizer, actor, tenantID, id string) error {
	if _, err := loadDocument(ctx, documents, tenantID, id); err != nil {
		return err
	}
	return authz.Require(ctx, actor, "can_share", model.ObjectTypeDocument, id)
}

func documentID(d model.Document) string { return d.ID }
