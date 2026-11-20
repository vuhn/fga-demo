package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type documentGrantLister struct {
	documents gateway.DocumentGateway
	grants    gateway.DocumentGrantGateway
	authz     helper.Authorizer
}

func NewDocumentGrantLister(documents gateway.DocumentGateway, grants gateway.DocumentGrantGateway, authz helper.Authorizer) usecase.DocumentGrantLister {
	return &documentGrantLister{documents: documents, grants: grants, authz: authz}
}

func (s *documentGrantLister) ListGrants(ctx context.Context, in input.DocumentGrantLister) (output.DocumentGrantLister, error) {
	if err := requireDocumentShare(ctx, s.documents, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return output.DocumentGrantLister{}, err
	}
	got, err := s.grants.List(ctx, in.ID)
	return output.DocumentGrantLister{Items: mapOutput(got, grantOutput)}, err
}
