package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func tenantOutput(m model.Tenant) output.Tenant {
	return output.Tenant{
		ID:        m.ID,
		Slug:      m.Slug,
		Name:      m.Name,
		CompanyID: m.CompanyID,
		CreatedAt: m.CreatedAt,
		Products:  mapOutput(m.Products, productOutput),
	}
}
