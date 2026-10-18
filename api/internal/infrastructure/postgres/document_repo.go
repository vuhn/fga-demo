package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type DocumentRepo struct{ pool *pgxpool.Pool }

var _ gateway.DocumentGateway = (*DocumentRepo)(nil)

func NewDocumentRepo(pool *pgxpool.Pool) *DocumentRepo { return &DocumentRepo{pool: pool} }

func (r *DocumentRepo) Create(ctx context.Context, d entity.Document, owner entity.Grant) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO documents (id, tenant_id, folder_id, name, mime_type, byte_size, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			d.ID, d.TenantID, d.FolderID, d.Name, d.MimeType, d.ByteSize, d.CreatedBy); err != nil {
			return err
		}
		return insertGrant(ctx, tx, entity.ObjectTypeDocument, owner)
	})
}

func (r *DocumentRepo) query(ctx context.Context, where string, args ...any) ([]entity.Document, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, folder_id, name, mime_type, byte_size, created_by, created_at FROM documents WHERE `+where+` ORDER BY name`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.Document{}
	for rows.Next() {
		var d entity.Document
		if err := rows.Scan(&d.ID, &d.TenantID, &d.FolderID, &d.Name, &d.MimeType, &d.ByteSize, &d.CreatedBy, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DocumentRepo) Get(ctx context.Context, id string) (entity.Document, error) {
	ds, err := r.query(ctx, `id = $1`, id)
	if err != nil {
		return entity.Document{}, err
	}
	if len(ds) == 0 {
		return entity.Document{}, entity.ErrNotFound
	}
	return ds[0], nil
}

func (r *DocumentRepo) ListByIDs(ctx context.Context, tenantID string, ids []string, folderID string) ([]entity.Document, error) {
	if folderID == "" {
		return r.query(ctx, `tenant_id = $1 AND id = ANY($2::uuid[])`, tenantID, ids)
	}
	return r.query(ctx, `tenant_id = $1 AND id = ANY($2::uuid[]) AND folder_id = $3`, tenantID, ids, folderID)
}

func (r *DocumentRepo) Rename(ctx context.Context, id, name string) (entity.Document, error) {
	if _, err := r.pool.Exec(ctx, `UPDATE documents SET name = $2 WHERE id = $1`, id, name); err != nil {
		return entity.Document{}, mapErr(err)
	}
	return r.Get(ctx, id)
}

func (r *DocumentRepo) Delete(ctx context.Context, id string) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := deleteObjectGrants(ctx, tx, entity.ObjectTypeDocument, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM documents WHERE id = $1`, id)
		return err
	})
}
