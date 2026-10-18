// Package helper declares the cross-cutting business rules the services
// share: write ordering and share validation.
package helper

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// WriteGuard keeps Postgres and OpenFGA fail-closed.
type WriteGuard interface {
	// Grant runs after Postgres has committed: it writes the tuples, and when
	// OpenFGA fails it runs undo and reports ErrTupleWriteFailed.
	Grant(ctx context.Context, writes, deletes []entity.Tuple, undo func(context.Context) error) error
	// Revoke deletes the tuples first, then runs persist. When persist fails
	// it writes restore back (exactly the tuples that existed).
	Revoke(ctx context.Context, deletes, restore []entity.Tuple, persist func(context.Context) error) error
	// RevokeObject revokes every tuple on the object, then runs persist.
	RevokeObject(ctx context.Context, objectType, objectID string, persist func(context.Context) error) error
}
