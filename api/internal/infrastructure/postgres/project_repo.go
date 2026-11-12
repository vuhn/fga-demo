package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type ProjectRepo struct{ pool *pgxpool.Pool }

var _ gateway.ProjectGateway = (*ProjectRepo)(nil)

func NewProjectRepo(pool *pgxpool.Pool) *ProjectRepo { return &ProjectRepo{pool: pool} }

func (r *ProjectRepo) Create(ctx context.Context, p entity.Project) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO projects (id, tenant_id, name, created_by) VALUES ($1, $2, $3, NULLIF($4, '')::uuid)`,
		p.ID, p.TenantID, p.Name, p.CreatedBy)
	return mapErr(err)
}

func (r *ProjectRepo) query(ctx context.Context, where string, args ...any) ([]entity.Project, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, name, COALESCE(created_by::text, ''), created_at FROM projects WHERE `+where+` ORDER BY name`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.Project{}
	for rows.Next() {
		var p entity.Project
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.CreatedBy, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *ProjectRepo) Get(ctx context.Context, id string) (entity.Project, error) {
	ps, err := r.query(ctx, `id = $1`, id)
	if err != nil {
		return entity.Project{}, err
	}
	if len(ps) == 0 {
		return entity.Project{}, entity.ErrNotFound
	}
	return ps[0], nil
}

func (r *ProjectRepo) ListByIDs(ctx context.Context, tenantID string, ids []string) ([]entity.Project, error) {
	return r.query(ctx, `tenant_id = $1 AND id = ANY($2::uuid[])`, tenantID, ids)
}

func (r *ProjectRepo) Rename(ctx context.Context, id, name string) (entity.Project, error) {
	if _, err := r.pool.Exec(ctx, `UPDATE projects SET name = $2 WHERE id = $1`, id, name); err != nil {
		return entity.Project{}, mapErr(err)
	}
	return r.Get(ctx, id)
}

func (r *ProjectRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	return mapErr(err)
}
