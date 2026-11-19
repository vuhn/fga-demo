package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
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
func (s *companyClaimer) Claim(ctx context.Context, actor, tenantID string, companyIDs []string) (model.Company, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return model.Company{}, err
	}
	if len(companyIDs) == 0 {
		return model.Company{}, model.Invalid("company_ids is required")
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return model.Company{}, err
	}
	own, err := s.companies.GetOwn(ctx, tenantID)
	if err != nil {
		return model.Company{}, err
	}
	seen := map[string]bool{}
	for _, id := range companyIDs {
		if err := model.RequireUUID("company_ids", id); err != nil {
			return model.Company{}, err
		}
		if seen[id] {
			return model.Company{}, model.Invalid("duplicate company id %s", id)
		}
		seen[id] = true
		c, err := s.companies.Get(ctx, id)
		if err != nil {
			return model.Company{}, err
		}
		if c.Claimed() {
			return model.Company{}, fmt.Errorf("%w: company %s is already claimed", model.ErrConflict, c.Name)
		}
	}
	n, err := s.companies.CountMembersIn(ctx, tenantID, companyIDs)
	if err != nil {
		return model.Company{}, err
	}
	if n > 0 {
		return model.Company{}, fmt.Errorf("%w: remove the claimer tenant's members of these companies first", model.ErrConflict)
	}
	if err := s.companies.Merge(ctx, own.ID, companyIDs); err != nil {
		return model.Company{}, err
	}
	return s.companies.Get(ctx, own.ID)
}
