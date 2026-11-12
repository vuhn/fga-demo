package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type allCompanyLister struct {
	companies gateway.CompanyGateway
	authz     usecase.Authorizer
}

func NewAllCompanyLister(companies gateway.CompanyGateway, authz usecase.Authorizer) usecase.AllCompanyLister {
	return &allCompanyLister{companies: companies, authz: authz}
}

// ListAll is platform only: it spans tenants.
func (s *allCompanyLister) ListAll(ctx context.Context, actor string) ([]entity.Company, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return nil, err
	}
	return s.companies.List(ctx)
}
