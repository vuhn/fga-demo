// Package idgen generates object ids.
package idgen

import (
	"github.com/google/uuid"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

// UUID returns random v4 UUIDs.
type UUID struct{}

var _ gateway.IDGenerator = UUID{}

func (UUID) NewID() string { return uuid.NewString() }
