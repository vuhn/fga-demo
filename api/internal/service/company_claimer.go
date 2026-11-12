package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type companyClaimer struct {
	tenants   gateway.TenantGateway
	companies gateway.CompanyGateway
	authz     usecase.Authorizer
}

func NewCompanyClaimer(tenants gateway.TenantGateway, companies gateway.CompanyGateway, authz usecase.Authorizer) usecase.CompanyClaimer {
	return &companyClaimer{tenants: tenants, companies: companies, authz: authz}
}

// Claim merges unclaimed companies (the company other tenants created for it,
// plus confirmed duplicates) into the claimer tenant's own company. The
// platform admin has verified ownership out of band. Memberships in other
// tenants are untouched: those users stay external there, so no tuple
// changes. A user already a member of the claimer tenant would flip
// affiliation, so that is a conflict.
func (s *companyClaimer) Claim(ctx context.Context, actor, tenantID string, companyIDs []string) (entity.Company, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return entity.Company{}, err
	}
	if len(companyIDs) == 0 {
		return entity.Company{}, entity.Invalid("company_ids is required")
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return entity.Company{}, err
	}
	own, err := s.companies.GetOwn(ctx, tenantID)
	if err != nil {
		return entity.Company{}, err
	}
	seen := map[string]bool{}
	for _, id := range companyIDs {
		if err := entity.RequireUUID("company_ids", id); err != nil {
			return entity.Company{}, err
		}
		if seen[id] {
			return entity.Company{}, entity.Invalid("duplicate company id %s", id)
		}
		seen[id] = true
		c, err := s.companies.Get(ctx, id)
		if err != nil {
			return entity.Company{}, err
		}
		if c.Claimed() {
			return entity.Company{}, fmt.Errorf("%w: company %s is already claimed", entity.ErrConflict, c.Name)
		}
	}
	n, err := s.companies.CountMembersIn(ctx, tenantID, companyIDs)
	if err != nil {
		return entity.Company{}, err
	}
	if n > 0 {
		return entity.Company{}, fmt.Errorf("%w: remove the claimer tenant's members of these companies first", entity.ErrConflict)
	}
	if err := s.companies.Merge(ctx, own.ID, companyIDs); err != nil {
		return entity.Company{}, err
	}
	return s.companies.Get(ctx, own.ID)
}
