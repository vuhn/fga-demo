package input

// MyTenantLister is the input of MyTenantLister.ListMine.
type MyTenantLister struct {
	Actor string
}

// TenantCreator is the input of TenantCreator.Create.
type TenantCreator struct {
	Actor string
	Name  string
	Slug  string
}

// TenantGetter is the input of TenantGetter.Get.
type TenantGetter struct {
	Actor string
	ID    string
}

// TenantLister is the input of TenantLister.List.
type TenantLister struct {
	Actor string
}

// TenantUpdater is the input of TenantUpdater.UpdateName.
type TenantUpdater struct {
	Actor string
	ID    string
	Name  string
}
