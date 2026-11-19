package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type PhotoGrantRepo struct{ pool *pgxpool.Pool }

var _ gateway.PhotoGrantGateway = (*PhotoGrantRepo)(nil)

func NewPhotoGrantRepo(pool *pgxpool.Pool) *PhotoGrantRepo { return &PhotoGrantRepo{pool: pool} }

func (r *PhotoGrantRepo) List(ctx context.Context, photoID string) ([]model.Grant, error) {
	return listObjectGrants(ctx, r.pool, model.ObjectTypePhoto, photoID)
}

func (r *PhotoGrantRepo) Upsert(ctx context.Context, g model.Grant) (*model.Grant, error) {
	return upsertGrant(ctx, r.pool, model.ObjectTypePhoto, g)
}

func (r *PhotoGrantRepo) Delete(ctx context.Context, id string) error {
	return deleteGrant(ctx, r.pool, model.ObjectTypePhoto, id)
}

func (r *PhotoGrantRepo) Restore(ctx context.Context, g model.Grant) error {
	return restoreGrant(ctx, r.pool, model.ObjectTypePhoto, g)
}
