package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type documentGrantLister struct {
	documents gateway.DocumentGateway
	grants    gateway.DocumentGrantGateway
	authz     usecase.Authorizer
}

func NewDocumentGrantLister(documents gateway.DocumentGateway, grants gateway.DocumentGrantGateway, authz usecase.Authorizer) usecase.DocumentGrantLister {
	return &documentGrantLister{documents: documents, grants: grants, authz: authz}
}

func (s *documentGrantLister) ListGrants(ctx context.Context, actor, tenantID, id string) ([]entity.Grant, error) {
	if err := requireDocumentShare(ctx, s.documents, s.authz, actor, tenantID, id); err != nil {
		return nil, err
	}
	return s.grants.List(ctx, id)
}
