package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type companyCreator struct {
	tenants   gateway.TenantGateway
	companies gateway.CompanyGateway
	authz     helper.Authorizer
	ids       gateway.IDGenerator
}

func NewCompanyCreator(tenants gateway.TenantGateway, companies gateway.CompanyGateway, authz helper.Authorizer, ids gateway.IDGenerator) usecase.CompanyCreator {
	return &companyCreator{tenants: tenants, companies: companies, authz: authz, ids: ids}
}

// Create adds an unclaimed subcontractor company on behalf of a tenant. The
// company is a display record: it has no tuples and grants nothing.
func (s *companyCreator) Create(ctx context.Context, in input.CompanyCreator) (output.CompanyCreator, error) {
	if err := s.authz.PlatformOr(ctx, in.Actor, "can_manage_members", in.TenantID, model.ErrForbidden); err != nil {
		return output.CompanyCreator{}, err
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.CompanyCreator{}, err
	}
	if _, err := s.tenants.Get(ctx, in.TenantID); err != nil {
		return output.CompanyCreator{}, err
	}
	c := model.Company{ID: s.ids.NewID(), Name: name, NameKey: model.CompanyNameKey(name), TaxID: strings.TrimSpace(in.TaxID), CreatedByTenantID: in.TenantID}
	if err := s.companies.Create(ctx, c); err != nil {
		return output.CompanyCreator{}, err
	}
	got, err := s.companies.Get(ctx, c.ID)
	return output.CompanyCreator{Company: companyOutput(got)}, err
}
