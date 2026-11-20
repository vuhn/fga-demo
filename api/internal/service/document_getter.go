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

type documentGetter struct {
	documents gateway.DocumentGateway
	authz     helper.Authorizer
}

func NewDocumentGetter(documents gateway.DocumentGateway, authz helper.Authorizer) usecase.DocumentGetter {
	return &documentGetter{documents: documents, authz: authz}
}

// Get is 404 when the caller cannot view the document.
func (s *documentGetter) Get(ctx context.Context, in input.DocumentGetter) (output.DocumentGetter, error) {
	d, err := loadDocument(ctx, s.documents, in.TenantID, in.ID)
	if err != nil {
		return output.DocumentGetter{}, err
	}
	if err := s.authz.RequireVisible(ctx, in.Actor, "can_view", model.ObjectTypeDocument, in.ID); err != nil {
		return output.DocumentGetter{}, err
	}
	vs, err := withPerms(ctx, s.authz, in.Actor, model.ObjectTypeDocument, []model.Document{d}, documentID)
	if err != nil {
		return output.DocumentGetter{}, err
	}
	return output.DocumentGetter{Visible: visibleOutput(vs[0], documentOutput)}, nil
}
