package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func companyOutput(m model.Company) output.Company {
	return output.Company{
		ID:                m.ID,
		Name:              m.Name,
		NameKey:           m.NameKey,
		TaxID:             m.TaxID,
		CreatedByTenantID: m.CreatedByTenantID,
		TenantID:          m.TenantID,
		ClaimedAt:         m.ClaimedAt,
		CreatedAt:         m.CreatedAt,
		MemberCount:       m.MemberCount,
		Claimed:           m.Claimed(),
	}
}
