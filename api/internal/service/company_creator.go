package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
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
func (s *companyCreator) Create(ctx context.Context, actor, tenantID string, in usecase.CreateCompanyInput) (entity.Company, error) {
	if err := s.authz.PlatformOr(ctx, actor, "can_manage_members", tenantID, entity.ErrForbidden); err != nil {
		return entity.Company{}, err
	}
	name, err := entity.CleanName("name", in.Name)
	if err != nil {
		return entity.Company{}, err
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return entity.Company{}, err
	}
	c := entity.Company{ID: s.ids.NewID(), Name: name, NameKey: entity.CompanyNameKey(name), TaxID: strings.TrimSpace(in.TaxID), CreatedByTenantID: tenantID}
	if err := s.companies.Create(ctx, c); err != nil {
		return entity.Company{}, err
	}
	return s.companies.Get(ctx, c.ID)
}
