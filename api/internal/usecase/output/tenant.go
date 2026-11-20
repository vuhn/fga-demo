package output

import (
	"time"
)

// MyTenantLister is the result of MyTenantLister.ListMine.
type MyTenantLister struct {
	Items []TenantView
}

// Tenant is the usecase output copy of a tenant.
type Tenant struct {
	ID        string
	Slug      string
	Name      string
	CompanyID string
	CreatedAt time.Time
	Products  []Product
}

// TenantCreator is the result of TenantCreator.Create.
type TenantCreator struct {
	Tenant
}

// TenantGetter is the result of TenantGetter.Get.
type TenantGetter struct {
	Tenant
}

// TenantLister is the result of TenantLister.List.
type TenantLister struct {
	Items []Tenant
}

// TenantUpdater is the result of TenantUpdater.UpdateName.
type TenantUpdater struct {
	Tenant
}

// TenantView is a tenant plus the actor's display labels for it.
type TenantView struct {
	Tenant
	Affiliation string
	Role        string
	RoleName    string
}
