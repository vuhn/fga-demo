package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type documentLister struct {
	documents gateway.DocumentGateway
	authz     usecase.Authorizer
}

func NewDocumentLister(documents gateway.DocumentGateway, authz usecase.Authorizer) usecase.DocumentLister {
	return &documentLister{documents: documents, authz: authz}
}

// List returns every visible document in the tenant, including documents
// whose folder the actor cannot see ("Shared with me").
func (s *documentLister) List(ctx context.Context, actor, tenantID, folderID string) ([]entity.Visible[entity.Document], error) {
	if folderID != "" {
		if err := entity.RequireUUID("folder_id", folderID); err != nil {
			return nil, err
		}
	}
	ids, err := s.authz.ListObjectIDs(ctx, actor, "can_view", entity.ObjectTypeDocument)
	if err != nil || len(ids) == 0 {
		return []entity.Visible[entity.Document]{}, err
	}
	documents, err := s.documents.ListByIDs(ctx, tenantID, ids, folderID)
	if err != nil {
		return nil, err
	}
	return withPerms(ctx, s.authz, actor, entity.ObjectTypeDocument, documents, documentID)
}
