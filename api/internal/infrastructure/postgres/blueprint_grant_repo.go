package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type BlueprintGrantRepo struct{ pool *pgxpool.Pool }

var _ gateway.BlueprintGrantGateway = (*BlueprintGrantRepo)(nil)

func NewBlueprintGrantRepo(pool *pgxpool.Pool) *BlueprintGrantRepo {
	return &BlueprintGrantRepo{pool: pool}
}

func (r *BlueprintGrantRepo) List(ctx context.Context, blueprintID string) ([]entity.Grant, error) {
	return listObjectGrants(ctx, r.pool, entity.ObjectTypeBlueprint, blueprintID)
}

func (r *BlueprintGrantRepo) Upsert(ctx context.Context, g entity.Grant) (*entity.Grant, error) {
	return upsertGrant(ctx, r.pool, entity.ObjectTypeBlueprint, g)
}

func (r *BlueprintGrantRepo) Delete(ctx context.Context, id string) error {
	return deleteGrant(ctx, r.pool, entity.ObjectTypeBlueprint, id)
}

func (r *BlueprintGrantRepo) Restore(ctx context.Context, g entity.Grant) error {
	return restoreGrant(ctx, r.pool, entity.ObjectTypeBlueprint, g)
}
