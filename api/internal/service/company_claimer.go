package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type companyClaimer struct {
	tenants   gateway.TenantGateway
	companies gateway.CompanyGateway
	authz     helper.Authorizer
}

func NewCompanyClaimer(tenants gateway.TenantGateway, companies gateway.CompanyGateway, authz helper.Authorizer) usecase.CompanyClaimer {
	return &companyClaimer{tenants: tenants, companies: companies, authz: authz}
}

// Claim merges unclaimed companies (the company other tenants created for it,
// plus confirmed duplicates) into the claimer tenant's own company. The
// platform admin has verified ownership out of band. Memberships in other
// tenants are untouched: those users stay external there, so no tuple
// changes. A user already a member of the claimer tenant would flip
// affiliation, so that is a conflict.
func (s *companyClaimer) Claim(ctx context.Context, in input.CompanyClaimer) (output.CompanyClaimer, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.CompanyClaimer{}, err
	}
	if len(in.CompanyIDs) == 0 {
		return output.CompanyClaimer{}, model.Invalid("company_ids is required")
	}
	if _, err := s.tenants.Get(ctx, in.TenantID); err != nil {
		return output.CompanyClaimer{}, err
	}
	own, err := s.companies.GetOwn(ctx, in.TenantID)
	if err != nil {
		return output.CompanyClaimer{}, err
	}
	seen := map[string]bool{}
	for _, id := range in.CompanyIDs {
		if err := model.RequireUUID("company_ids", id); err != nil {
			return output.CompanyClaimer{}, err
		}
		if seen[id] {
			return output.CompanyClaimer{}, model.Invalid("duplicate company id %s", id)
		}
		seen[id] = true
		c, err := s.companies.Get(ctx, id)
		if err != nil {
			return output.CompanyClaimer{}, err
		}
		if c.Claimed() {
			return output.CompanyClaimer{}, fmt.Errorf("%w: company %s is already claimed", model.ErrConflict, c.Name)
		}
	}
	n, err := s.companies.CountMembersIn(ctx, in.TenantID, in.CompanyIDs)
	if err != nil {
		return output.CompanyClaimer{}, err
	}
	if n > 0 {
		return output.CompanyClaimer{}, fmt.Errorf("%w: remove the claimer tenant's members of these companies first", model.ErrConflict)
	}
	if err := s.companies.Merge(ctx, own.ID, in.CompanyIDs); err != nil {
		return output.CompanyClaimer{}, err
	}
	got, err := s.companies.Get(ctx, own.ID)
	return output.CompanyClaimer{Company: companyOutput(got)}, err
}
