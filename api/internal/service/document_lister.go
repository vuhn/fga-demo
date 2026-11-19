package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
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
func (s *documentLister) List(ctx context.Context, actor, tenantID, folderID, projectID string) ([]model.Visible[model.Document], error) {
	if folderID != "" {
		if err := model.RequireUUID("folder_id", folderID); err != nil {
			return nil, err
		}
	}
	if projectID != "" {
		if err := model.RequireUUID("project_id", projectID); err != nil {
			return nil, err
		}
	}
	ids, err := s.authz.ListObjectIDs(ctx, actor, "can_view", model.ObjectTypeDocument)
	if err != nil || len(ids) == 0 {
		return []model.Visible[model.Document]{}, err
	}
	documents, err := s.documents.ListByIDs(ctx, tenantID, ids, folderID, projectID)
	if err != nil {
		return nil, err
	}
	return withPerms(ctx, s.authz, actor, model.ObjectTypeDocument, documents, documentID)
}
