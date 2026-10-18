package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type FolderGrantRepo struct{ pool *pgxpool.Pool }

var _ gateway.FolderGrantGateway = (*FolderGrantRepo)(nil)

func NewFolderGrantRepo(pool *pgxpool.Pool) *FolderGrantRepo { return &FolderGrantRepo{pool: pool} }

func (r *FolderGrantRepo) List(ctx context.Context, folderID string) ([]entity.Grant, error) {
	return listObjectGrants(ctx, r.pool, entity.ObjectTypeFolder, folderID)
}

func (r *FolderGrantRepo) Upsert(ctx context.Context, g entity.Grant) (*entity.Grant, error) {
	return upsertGrant(ctx, r.pool, entity.ObjectTypeFolder, g)
}

func (r *FolderGrantRepo) Delete(ctx context.Context, id string) error {
	return deleteGrant(ctx, r.pool, entity.ObjectTypeFolder, id)
}

func (r *FolderGrantRepo) Restore(ctx context.Context, g entity.Grant) error {
	return restoreGrant(ctx, r.pool, entity.ObjectTypeFolder, g)
}
