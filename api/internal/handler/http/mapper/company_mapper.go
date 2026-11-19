package mapper

import (
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type CompanyJSON struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	TaxID             string     `json:"tax_id,omitempty"`
	CreatedByTenantID string     `json:"created_by_tenant_id,omitempty"`
	TenantID          string     `json:"tenant_id,omitempty"`
	Claimed           bool       `json:"claimed"`
	ClaimedAt         *time.Time `json:"claimed_at"`
	MemberCount       int        `json:"member_count"`
	CreatedAt         time.Time  `json:"created_at"`
}

func CompanyView(c model.Company) CompanyJSON {
	return CompanyJSON{c.ID, c.Name, c.TaxID, c.CreatedByTenantID, c.TenantID, c.Claimed(), c.ClaimedAt, c.MemberCount, c.CreatedAt}
}
