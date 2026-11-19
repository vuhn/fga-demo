package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type BlueprintGrantRepo struct{ pool *pgxpool.Pool }

var _ gateway.BlueprintGrantGateway = (*BlueprintGrantRepo)(nil)

func NewBlueprintGrantRepo(pool *pgxpool.Pool) *BlueprintGrantRepo {
	return &BlueprintGrantRepo{pool: pool}
}

func (r *BlueprintGrantRepo) List(ctx context.Context, blueprintID string) ([]model.Grant, error) {
	return listObjectGrants(ctx, r.pool, model.ObjectTypeBlueprint, blueprintID)
}

func (r *BlueprintGrantRepo) Upsert(ctx context.Context, g model.Grant) (*model.Grant, error) {
	return upsertGrant(ctx, r.pool, model.ObjectTypeBlueprint, g)
}

func (r *BlueprintGrantRepo) Delete(ctx context.Context, id string) error {
	return deleteGrant(ctx, r.pool, model.ObjectTypeBlueprint, id)
}

func (r *BlueprintGrantRepo) Restore(ctx context.Context, g model.Grant) error {
	return restoreGrant(ctx, r.pool, model.ObjectTypeBlueprint, g)
}
