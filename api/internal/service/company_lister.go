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

type companyLister struct {
	tenants   gateway.TenantGateway
	companies gateway.CompanyGateway
	authz     helper.Authorizer
}

func NewCompanyLister(tenants gateway.TenantGateway, companies gateway.CompanyGateway, authz helper.Authorizer) usecase.CompanyLister {
	return &companyLister{tenants: tenants, companies: companies, authz: authz}
}

// List is tenant administration: member managers need it to place new members.
func (s *companyLister) List(ctx context.Context, in input.CompanyLister) (output.CompanyLister, error) {
	if err := s.authz.PlatformOr(ctx, in.Actor, "can_manage_members", in.TenantID, model.ErrForbidden); err != nil {
		return output.CompanyLister{}, err
	}
	if _, err := s.tenants.Get(ctx, in.TenantID); err != nil {
		return output.CompanyLister{}, err
	}
	got, err := s.companies.ListForTenant(ctx, in.TenantID)
	return output.CompanyLister{Items: mapOutput(got, companyOutput)}, err
}
