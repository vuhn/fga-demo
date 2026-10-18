package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type PhotoGrantRepo struct{ pool *pgxpool.Pool }

var _ gateway.PhotoGrantGateway = (*PhotoGrantRepo)(nil)

func NewPhotoGrantRepo(pool *pgxpool.Pool) *PhotoGrantRepo { return &PhotoGrantRepo{pool: pool} }

func (r *PhotoGrantRepo) List(ctx context.Context, photoID string) ([]entity.Grant, error) {
	return listObjectGrants(ctx, r.pool, entity.ObjectTypePhoto, photoID)
}

func (r *PhotoGrantRepo) Upsert(ctx context.Context, g entity.Grant) (*entity.Grant, error) {
	return upsertGrant(ctx, r.pool, entity.ObjectTypePhoto, g)
}

func (r *PhotoGrantRepo) Delete(ctx context.Context, id string) error {
	return deleteGrant(ctx, r.pool, entity.ObjectTypePhoto, id)
}

func (r *PhotoGrantRepo) Restore(ctx context.Context, g entity.Grant) error {
	return restoreGrant(ctx, r.pool, entity.ObjectTypePhoto, g)
}
