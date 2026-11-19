package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// AuthzGateway is the OpenFGA store: the only component that answers allow
// or deny.
type AuthzGateway interface {
	Check(ctx context.Context, userID, relation, objectType, objectID string) (bool, error)
	BatchCheck(ctx context.Context, checks []model.CheckRequest) ([]bool, error)
	ListObjectIDs(ctx context.Context, userID, relation, objectType string) ([]string, error)
	ReadTuples(ctx context.Context, objectType, objectID string) ([]model.Tuple, error)
	// Write applies writes and deletes atomically. Writing an existing tuple
	// and deleting a missing one both succeed.
	Write(ctx context.Context, writes, deletes []model.Tuple) error
}
