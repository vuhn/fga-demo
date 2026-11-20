package input

// ProductLister is the input of ProductLister.List.
type ProductLister struct {
	Actor    string
	TenantID string
}

// ProductSetter is the input of ProductSetter.Set.
type ProductSetter struct {
	Actor    string
	TenantID string
	Product  string
	Enabled  bool
}
