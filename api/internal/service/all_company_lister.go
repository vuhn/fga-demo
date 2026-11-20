package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type allCompanyLister struct {
	companies gateway.CompanyGateway
	authz     helper.Authorizer
}

func NewAllCompanyLister(companies gateway.CompanyGateway, authz helper.Authorizer) usecase.AllCompanyLister {
	return &allCompanyLister{companies: companies, authz: authz}
}

// ListAll is platform only: it spans tenants.
func (s *allCompanyLister) ListAll(ctx context.Context, in input.AllCompanyLister) (output.AllCompanyLister, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.AllCompanyLister{}, err
	}
	got, err := s.companies.List(ctx)
	return output.AllCompanyLister{Items: mapOutput(got, companyOutput)}, err
}
