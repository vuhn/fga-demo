package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type AlbumGrantRepo struct{ pool *pgxpool.Pool }

var _ gateway.AlbumGrantGateway = (*AlbumGrantRepo)(nil)

func NewAlbumGrantRepo(pool *pgxpool.Pool) *AlbumGrantRepo { return &AlbumGrantRepo{pool: pool} }

func (r *AlbumGrantRepo) List(ctx context.Context, albumID string) ([]entity.Grant, error) {
	return listObjectGrants(ctx, r.pool, entity.ObjectTypeAlbum, albumID)
}

func (r *AlbumGrantRepo) Upsert(ctx context.Context, g entity.Grant) (*entity.Grant, error) {
	return upsertGrant(ctx, r.pool, entity.ObjectTypeAlbum, g)
}

func (r *AlbumGrantRepo) Delete(ctx context.Context, id string) error {
	return deleteGrant(ctx, r.pool, entity.ObjectTypeAlbum, id)
}

func (r *AlbumGrantRepo) Restore(ctx context.Context, g entity.Grant) error {
	return restoreGrant(ctx, r.pool, entity.ObjectTypeAlbum, g)
}
