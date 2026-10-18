package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type DocumentGrantRepo struct{ pool *pgxpool.Pool }

var _ gateway.DocumentGrantGateway = (*DocumentGrantRepo)(nil)

func NewDocumentGrantRepo(pool *pgxpool.Pool) *DocumentGrantRepo {
	return &DocumentGrantRepo{pool: pool}
}

func (r *DocumentGrantRepo) List(ctx context.Context, documentID string) ([]entity.Grant, error) {
	return listObjectGrants(ctx, r.pool, entity.ObjectTypeDocument, documentID)
}

func (r *DocumentGrantRepo) Upsert(ctx context.Context, g entity.Grant) (*entity.Grant, error) {
	return upsertGrant(ctx, r.pool, entity.ObjectTypeDocument, g)
}

func (r *DocumentGrantRepo) Delete(ctx context.Context, id string) error {
	return deleteGrant(ctx, r.pool, entity.ObjectTypeDocument, id)
}

func (r *DocumentGrantRepo) Restore(ctx context.Context, g entity.Grant) error {
	return restoreGrant(ctx, r.pool, entity.ObjectTypeDocument, g)
}
