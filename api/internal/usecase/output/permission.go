package output

// PermissionReader is the caller's platform permissions and, when a tenant is
// given, their permissions on that tenant.
type PermissionReader struct {
	Platform map[string]bool
	TenantID string
	Tenant   map[string]bool // nil when no tenant was requested
}
