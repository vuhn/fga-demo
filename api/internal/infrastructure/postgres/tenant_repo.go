package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type TenantRepo struct{ pool *pgxpool.Pool }

var _ gateway.TenantGateway = (*TenantRepo)(nil)

func NewTenantRepo(pool *pgxpool.Pool) *TenantRepo { return &TenantRepo{pool: pool} }

func (r *TenantRepo) Create(ctx context.Context, t entity.Tenant) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, slug, name) VALUES ($1, $2, $3)`, t.ID, t.Slug, t.Name); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO tenant_products (tenant_id, product, enabled) VALUES ($1, 'documents', false), ($1, 'photos', false)`, t.ID)
		return err
	})
}

// query loads tenants with their products.
func (r *TenantRepo) query(ctx context.Context, q string, args ...any) ([]entity.Tenant, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.Tenant{}
	for rows.Next() {
		var t entity.Tenant
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Products, err = listProducts(ctx, r.pool, out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *TenantRepo) Get(ctx context.Context, id string) (entity.Tenant, error) {
	ts, err := r.query(ctx, `SELECT id, slug, name, created_at FROM tenants WHERE id = $1`, id)
	if err != nil {
		return entity.Tenant{}, err
	}
	if len(ts) == 0 {
		return entity.Tenant{}, entity.ErrNotFound
	}
	return ts[0], nil
}

func (r *TenantRepo) List(ctx context.Context) ([]entity.Tenant, error) {
	return r.query(ctx, `SELECT id, slug, name, created_at FROM tenants ORDER BY name`)
}

func (r *TenantRepo) ListByIDs(ctx context.Context, ids []string) ([]entity.Tenant, error) {
	return r.query(ctx, `SELECT id, slug, name, created_at FROM tenants WHERE id = ANY($1::uuid[]) ORDER BY name`, ids)
}

func (r *TenantRepo) UpdateName(ctx context.Context, id, name string) (entity.Tenant, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE tenants SET name = $2 WHERE id = $1`, id, name)
	if err != nil {
		return entity.Tenant{}, mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return entity.Tenant{}, entity.ErrNotFound
	}
	return r.Get(ctx, id)
}
