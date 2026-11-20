package output

import (
	"time"
)

// AllCompanyLister is the result of AllCompanyLister.ListAll.
type AllCompanyLister struct {
	Items []Company
}

// Company is the usecase output copy of a company.
type Company struct {
	ID                string
	Name              string
	NameKey           string
	TaxID             string
	CreatedByTenantID string
	TenantID          string
	ClaimedAt         *time.Time
	CreatedAt         time.Time
	MemberCount       int
	// Claimed reports whether a tenant has claimed the company.
	Claimed bool
}

// CompanyClaimer is the result of CompanyClaimer.Claim.
type CompanyClaimer struct {
	Company
}

// CompanyCreator is the result of CompanyCreator.Create.
type CompanyCreator struct {
	Company
}

// CompanyLister is the result of CompanyLister.List.
type CompanyLister struct {
	Items []Company
}
