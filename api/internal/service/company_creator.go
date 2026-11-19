package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type companyCreator struct {
	tenants   gateway.TenantGateway
	companies gateway.CompanyGateway
	authz     usecase.Authorizer
	ids       gateway.IDGenerator
}

func NewCompanyCreator(tenants gateway.TenantGateway, companies gateway.CompanyGateway, authz usecase.Authorizer, ids gateway.IDGenerator) usecase.CompanyCreator {
	return &companyCreator{tenants: tenants, companies: companies, authz: authz, ids: ids}
}

// Create adds an unclaimed subcontractor company on behalf of a tenant. The
// company is a display record: it has no tuples and grants nothing.
func (s *companyCreator) Create(ctx context.Context, actor, tenantID string, in usecase.CreateCompanyInput) (model.Company, error) {
	if err := s.authz.PlatformOr(ctx, actor, "can_manage_members", tenantID, model.ErrForbidden); err != nil {
		return model.Company{}, err
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return model.Company{}, err
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return model.Company{}, err
	}
	c := model.Company{ID: s.ids.NewID(), Name: name, NameKey: model.CompanyNameKey(name), TaxID: strings.TrimSpace(in.TaxID), CreatedByTenantID: tenantID}
	if err := s.companies.Create(ctx, c); err != nil {
		return model.Company{}, err
	}
	return s.companies.Get(ctx, c.ID)
}
