package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type AlbumGrantRepo struct{ pool *pgxpool.Pool }

var _ gateway.AlbumGrantGateway = (*AlbumGrantRepo)(nil)

func NewAlbumGrantRepo(pool *pgxpool.Pool) *AlbumGrantRepo { return &AlbumGrantRepo{pool: pool} }

func (r *AlbumGrantRepo) List(ctx context.Context, albumID string) ([]model.Grant, error) {
	return listObjectGrants(ctx, r.pool, model.ObjectTypeAlbum, albumID)
}

func (r *AlbumGrantRepo) Upsert(ctx context.Context, g model.Grant) (*model.Grant, error) {
	return upsertGrant(ctx, r.pool, model.ObjectTypeAlbum, g)
}

func (r *AlbumGrantRepo) Delete(ctx context.Context, id string) error {
	return deleteGrant(ctx, r.pool, model.ObjectTypeAlbum, id)
}

func (r *AlbumGrantRepo) Restore(ctx context.Context, g model.Grant) error {
	return restoreGrant(ctx, r.pool, model.ObjectTypeAlbum, g)
}
