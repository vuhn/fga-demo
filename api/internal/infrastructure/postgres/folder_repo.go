package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type FolderRepo struct{ pool *pgxpool.Pool }

var _ gateway.FolderGateway = (*FolderRepo)(nil)

func NewFolderRepo(pool *pgxpool.Pool) *FolderRepo { return &FolderRepo{pool: pool} }

func (r *FolderRepo) Create(ctx context.Context, f entity.Folder, owner entity.Grant) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO folders (id, tenant_id, parent_id, name, is_private, created_by) VALUES ($1, $2, $3, $4, $5, $6)`,
			f.ID, f.TenantID, f.ParentID, f.Name, f.IsPrivate, f.CreatedBy); err != nil {
			return err
		}
		return insertGrant(ctx, tx, entity.ObjectTypeFolder, owner)
	})
}

func (r *FolderRepo) query(ctx context.Context, where string, args ...any) ([]entity.Folder, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, parent_id, name, is_private, created_by, created_at FROM folders WHERE `+where+` ORDER BY name`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.Folder{}
	for rows.Next() {
		var f entity.Folder
		if err := rows.Scan(&f.ID, &f.TenantID, &f.ParentID, &f.Name, &f.IsPrivate, &f.CreatedBy, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *FolderRepo) Get(ctx context.Context, id string) (entity.Folder, error) {
	fs, err := r.query(ctx, `id = $1`, id)
	if err != nil {
		return entity.Folder{}, err
	}
	if len(fs) == 0 {
		return entity.Folder{}, entity.ErrNotFound
	}
	return fs[0], nil
}

func (r *FolderRepo) ListByIDs(ctx context.Context, tenantID string, ids []string) ([]entity.Folder, error) {
	return r.query(ctx, `tenant_id = $1 AND id = ANY($2::uuid[])`, tenantID, ids)
}

func (r *FolderRepo) Rename(ctx context.Context, id, name string) (entity.Folder, error) {
	if _, err := r.pool.Exec(ctx, `UPDATE folders SET name = $2 WHERE id = $1`, id, name); err != nil {
		return entity.Folder{}, mapErr(err)
	}
	return r.Get(ctx, id)
}

func (r *FolderRepo) SetPrivate(ctx context.Context, id string, isPrivate bool) (bool, error) {
	var prev bool
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT is_private FROM folders WHERE id = $1 FOR UPDATE`, id).Scan(&prev); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE folders SET is_private = $2 WHERE id = $1`, id, isPrivate)
		return err
	})
	return prev, err
}

func (r *FolderRepo) HasChildren(ctx context.Context, id string) (bool, error) {
	var has bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM folders WHERE parent_id = $1) OR EXISTS (SELECT 1 FROM documents WHERE folder_id = $1)`, id).Scan(&has)
	return has, mapErr(err)
}

func (r *FolderRepo) Delete(ctx context.Context, id string) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := deleteObjectGrants(ctx, tx, entity.ObjectTypeFolder, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM folders WHERE id = $1`, id)
		return err
	})
}
