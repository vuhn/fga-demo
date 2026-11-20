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

type documentRenamer struct {
	documents gateway.DocumentGateway
	authz     helper.Authorizer
}

func NewDocumentRenamer(documents gateway.DocumentGateway, authz helper.Authorizer) usecase.DocumentRenamer {
	return &documentRenamer{documents: documents, authz: authz}
}

// Rename requires can_edit.
func (s *documentRenamer) Rename(ctx context.Context, in input.DocumentRenamer) (output.DocumentRenamer, error) {
	if _, err := loadDocument(ctx, s.documents, in.TenantID, in.ID); err != nil {
		return output.DocumentRenamer{}, err
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.DocumentRenamer{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_edit", model.ObjectTypeDocument, in.ID); err != nil {
		return output.DocumentRenamer{}, err
	}
	got, err := s.documents.Rename(ctx, in.ID, name)
	return output.DocumentRenamer{Document: documentOutput(got)}, err
}
