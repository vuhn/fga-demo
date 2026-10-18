package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// ProductGateway stores the display flag of each tenant product. The
// entitlement itself is the product's wildcard tuple.
type ProductGateway interface {
	List(ctx context.Context, tenantID string) ([]entity.Product, error)
	// Set stores the display flag and returns the previous value.
	Set(ctx context.Context, tenantID, product string, enabled bool) (prev bool, err error)
}

// ProductRelationGateway builds the product tuples. It is pure.
type ProductRelationGateway interface {
	// Enabled is the user:* wildcard tuple that switches the product on.
	Enabled(tenantID, product string) entity.Tuple
}
