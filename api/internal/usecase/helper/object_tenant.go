package helper

import "context"

// ObjectTenantResolver returns the tenant of any content object, so the
// inspector derives the tenant from the object, never from the request.
type ObjectTenantResolver interface {
	TenantOf(ctx context.Context, objectType, objectID string) (string, error)
}
