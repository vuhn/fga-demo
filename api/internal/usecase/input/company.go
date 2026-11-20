package input

// AllCompanyLister is the input of AllCompanyLister.ListAll.
type AllCompanyLister struct {
	Actor string
}

// CompanyClaimer is the input of CompanyClaimer.Claim.
type CompanyClaimer struct {
	Actor      string
	TenantID   string
	CompanyIDs []string
}

// CompanyCreator is the input of CompanyCreator.Create.
type CompanyCreator struct {
	Actor    string
	TenantID string
	Name     string
	TaxID    string
}

// CompanyLister is the input of CompanyLister.List.
type CompanyLister struct {
	Actor    string
	TenantID string
}
