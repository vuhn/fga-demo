package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// Inspector evaluates Checks about any user after authorizing each one.
type Inspector interface {
	Inspect(ctx context.Context, actor string, c entity.CheckRequest) (bool, error)
	BatchInspect(ctx context.Context, actor string, checks []entity.CheckRequest) ([]bool, error)
}

type MemberAccess struct {
	UserID    string
	Name      string
	CanView   bool
	CanEdit   bool
	CanDelete bool
}

type AccessReport struct {
	Object       string
	DirectTuples []entity.Tuple
	Members      []MemberAccess
}

// AccessReporter lists the direct tuples on a content object and what every
// tenant member and platform admin may do with it.
type AccessReporter interface {
	Report(ctx context.Context, actor, objectType, objectID string) (AccessReport, error)
}
