package helper

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// SharePolicy holds the share rules every content type follows.
type SharePolicy interface {
	// NewGrant validates the input and builds the grant on objectType:objectID.
	// Only viewer and editor can be shared; owner never can.
	NewGrant(tenantID, objectType, objectID string, in entity.ShareInput) (entity.Grant, error)
	// CheckGrantee requires a direct user grantee to read the tenant and, for
	// editor, to be able to contribute (externals cannot upload).
	CheckGrantee(ctx context.Context, tenantID string, g entity.Grant) error
}
