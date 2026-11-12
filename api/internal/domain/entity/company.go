package entity

import (
	"strings"
	"time"
)

// Company is the employer of a user. Every tenant owns one company (TenantID
// set); a subcontractor created by a tenant is unclaimed (TenantID empty)
// until it signs its own contract and a platform admin claims it.
type Company struct {
	ID                string
	Name              string
	NameKey           string
	TaxID             string
	CreatedByTenantID string
	// TenantID is set only on a tenant's own company.
	TenantID    string
	ClaimedAt   *time.Time
	CreatedAt   time.Time
	MemberCount int
}

// CompanyNameKey normalizes a name for duplicate detection.
func CompanyNameKey(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), " ")
}

// Claimed reports whether the company is a tenant's own company.
func (c Company) Claimed() bool { return c.TenantID != "" }

// DeriveAffiliation is the single rule behind a member's affiliation: a
// member is internal when their company is the tenant's own company.
func DeriveAffiliation(company Company, tenantID string) string {
	if company.TenantID != "" && company.TenantID == tenantID {
		return AffiliationInternal
	}
	return AffiliationExternal
}
