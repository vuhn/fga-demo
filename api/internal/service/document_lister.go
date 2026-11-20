package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type documentLister struct {
	documents gateway.DocumentGateway
	authz     helper.Authorizer
}

func NewDocumentLister(documents gateway.DocumentGateway, authz helper.Authorizer) usecase.DocumentLister {
	return &documentLister{documents: documents, authz: authz}
}

// List returns every visible document in the tenant, including documents
// whose folder the actor cannot see ("Shared with me").
func (s *documentLister) List(ctx context.Context, in input.DocumentLister) (output.DocumentLister, error) {
	if in.FolderID != "" {
		if err := model.RequireUUID("folder_id", in.FolderID); err != nil {
			return output.DocumentLister{}, err
		}
	}
	if in.ProjectID != "" {
		if err := model.RequireUUID("project_id", in.ProjectID); err != nil {
			return output.DocumentLister{}, err
		}
	}
	ids, err := s.authz.ListObjectIDs(ctx, in.Actor, "can_view", model.ObjectTypeDocument)
	if err != nil || len(ids) == 0 {
		return output.DocumentLister{}, err
	}
	documents, err := s.documents.ListByIDs(ctx, model.ListDocumentsParams{TenantID: in.TenantID, IDs: ids, FolderID: in.FolderID, ProjectID: in.ProjectID})
	if err != nil {
		return output.DocumentLister{}, err
	}
	got, err := withPerms(ctx, s.authz, in.Actor, model.ObjectTypeDocument, documents, documentID)
	return output.DocumentLister{Items: mapVisibleOutput(got, documentOutput)}, err
}
