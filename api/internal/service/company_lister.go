package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type companyLister struct {
	tenants   gateway.TenantGateway
	companies gateway.CompanyGateway
	authz     usecase.Authorizer
}

func NewCompanyLister(tenants gateway.TenantGateway, companies gateway.CompanyGateway, authz usecase.Authorizer) usecase.CompanyLister {
	return &companyLister{tenants: tenants, companies: companies, authz: authz}
}

// List is tenant administration: member managers need it to place new members.
func (s *companyLister) List(ctx context.Context, actor, tenantID string) ([]entity.Company, error) {
	if err := s.authz.PlatformOr(ctx, actor, "can_manage_members", tenantID, entity.ErrForbidden); err != nil {
		return nil, err
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return nil, err
	}
	return s.companies.ListForTenant(ctx, tenantID)
}
