package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type FolderGrantRepo struct{ pool *pgxpool.Pool }

var _ gateway.FolderGrantGateway = (*FolderGrantRepo)(nil)

func NewFolderGrantRepo(pool *pgxpool.Pool) *FolderGrantRepo { return &FolderGrantRepo{pool: pool} }

func (r *FolderGrantRepo) List(ctx context.Context, folderID string) ([]model.Grant, error) {
	return listObjectGrants(ctx, r.pool, model.ObjectTypeFolder, folderID)
}

func (r *FolderGrantRepo) Upsert(ctx context.Context, g model.Grant) (*model.Grant, error) {
	return upsertGrant(ctx, r.pool, model.ObjectTypeFolder, g)
}

func (r *FolderGrantRepo) Delete(ctx context.Context, id string) error {
	return deleteGrant(ctx, r.pool, model.ObjectTypeFolder, id)
}

func (r *FolderGrantRepo) Restore(ctx context.Context, g model.Grant) error {
	return restoreGrant(ctx, r.pool, model.ObjectTypeFolder, g)
}
