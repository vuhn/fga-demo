package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type PhotoRepo struct{ pool *pgxpool.Pool }

var _ gateway.PhotoGateway = (*PhotoRepo)(nil)

func NewPhotoRepo(pool *pgxpool.Pool) *PhotoRepo { return &PhotoRepo{pool: pool} }

func (r *PhotoRepo) Create(ctx context.Context, p entity.Photo, owner entity.Grant) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO photos (id, tenant_id, project_id, album_id, name, mime_type, byte_size, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			p.ID, p.TenantID, p.ProjectID, p.AlbumID, p.Name, p.MimeType, p.ByteSize, p.CreatedBy); err != nil {
			return err
		}
		return insertGrant(ctx, tx, entity.ObjectTypePhoto, owner)
	})
}

func (r *PhotoRepo) query(ctx context.Context, where string, args ...any) ([]entity.Photo, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, project_id, album_id, name, mime_type, byte_size, created_by, created_at FROM photos WHERE `+where+` ORDER BY name`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.Photo{}
	for rows.Next() {
		var p entity.Photo
		if err := rows.Scan(&p.ID, &p.TenantID, &p.ProjectID, &p.AlbumID, &p.Name, &p.MimeType, &p.ByteSize, &p.CreatedBy, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PhotoRepo) Get(ctx context.Context, id string) (entity.Photo, error) {
	ps, err := r.query(ctx, `id = $1`, id)
	if err != nil {
		return entity.Photo{}, err
	}
	if len(ps) == 0 {
		return entity.Photo{}, entity.ErrNotFound
	}
	return ps[0], nil
}

func (r *PhotoRepo) ListByIDs(ctx context.Context, tenantID string, ids []string, albumID, projectID string) ([]entity.Photo, error) {
	return r.query(ctx, `tenant_id = $1 AND id = ANY($2::uuid[]) AND ($3 = '' OR album_id::text = $3) AND ($4 = '' OR project_id::text = $4)`, tenantID, ids, albumID, projectID)
}

func (r *PhotoRepo) Rename(ctx context.Context, id, name string) (entity.Photo, error) {
	if _, err := r.pool.Exec(ctx, `UPDATE photos SET name = $2 WHERE id = $1`, id, name); err != nil {
		return entity.Photo{}, mapErr(err)
	}
	return r.Get(ctx, id)
}

func (r *PhotoRepo) Delete(ctx context.Context, id string) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := deleteObjectGrants(ctx, tx, entity.ObjectTypePhoto, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM photos WHERE id = $1`, id)
		return err
	})
}
