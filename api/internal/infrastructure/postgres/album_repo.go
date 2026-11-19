package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type AlbumRepo struct{ pool *pgxpool.Pool }

var _ gateway.AlbumGateway = (*AlbumRepo)(nil)

func NewAlbumRepo(pool *pgxpool.Pool) *AlbumRepo { return &AlbumRepo{pool: pool} }

func (r *AlbumRepo) Create(ctx context.Context, a model.Album, owner model.Grant) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO albums (id, tenant_id, project_id, parent_id, name, is_private, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			a.ID, a.TenantID, a.ProjectID, a.ParentID, a.Name, a.IsPrivate, a.CreatedBy); err != nil {
			return err
		}
		return insertGrant(ctx, tx, model.ObjectTypeAlbum, owner)
	})
}

func (r *AlbumRepo) query(ctx context.Context, where string, args ...any) ([]model.Album, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, project_id, parent_id, name, is_private, created_by, created_at FROM albums WHERE `+where+` ORDER BY name`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []model.Album{}
	for rows.Next() {
		var a model.Album
		if err := rows.Scan(&a.ID, &a.TenantID, &a.ProjectID, &a.ParentID, &a.Name, &a.IsPrivate, &a.CreatedBy, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *AlbumRepo) Get(ctx context.Context, id string) (model.Album, error) {
	as, err := r.query(ctx, `id = $1`, id)
	if err != nil {
		return model.Album{}, err
	}
	if len(as) == 0 {
		return model.Album{}, model.ErrNotFound
	}
	return as[0], nil
}

func (r *AlbumRepo) ListByIDs(ctx context.Context, tenantID string, ids []string, projectID string) ([]model.Album, error) {
	return r.query(ctx, `tenant_id = $1 AND id = ANY($2::uuid[]) AND ($3 = '' OR project_id::text = $3)`, tenantID, ids, projectID)
}

func (r *AlbumRepo) Rename(ctx context.Context, id, name string) (model.Album, error) {
	if _, err := r.pool.Exec(ctx, `UPDATE albums SET name = $2 WHERE id = $1`, id, name); err != nil {
		return model.Album{}, mapErr(err)
	}
	return r.Get(ctx, id)
}

func (r *AlbumRepo) SetPrivate(ctx context.Context, id string, isPrivate bool) (bool, error) {
	var prev bool
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT is_private FROM albums WHERE id = $1 FOR UPDATE`, id).Scan(&prev); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE albums SET is_private = $2 WHERE id = $1`, id, isPrivate)
		return err
	})
	return prev, err
}

func (r *AlbumRepo) HasChildren(ctx context.Context, id string) (bool, error) {
	var has bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM albums WHERE parent_id = $1) OR EXISTS (SELECT 1 FROM photos WHERE album_id = $1)`, id).Scan(&has)
	return has, mapErr(err)
}

func (r *AlbumRepo) Delete(ctx context.Context, id string) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := deleteObjectGrants(ctx, tx, model.ObjectTypeAlbum, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM albums WHERE id = $1`, id)
		return err
	})
}
