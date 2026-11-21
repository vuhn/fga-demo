package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type ProductRepo struct{ pool *pgxpool.Pool }

var _ gateway.ProductGateway = (*ProductRepo)(nil)

func NewProductRepo(pool *pgxpool.Pool) *ProductRepo { return &ProductRepo{pool: pool} }

func (r *ProductRepo) List(ctx context.Context, tenantID string) ([]model.Product, error) {
	return listProducts(ctx, r.pool, tenantID)
}

func listProducts(ctx context.Context, pool *pgxpool.Pool, tenantID string) ([]model.Product, error) {
	rows, err := pool.Query(ctx, `SELECT product, enabled, updated_at FROM tenant_products WHERE tenant_id = $1 ORDER BY product`, tenantID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []model.Product{}
	for rows.Next() {
		var p model.Product
		if err := rows.Scan(&p.Name, &p.Enabled, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *ProductRepo) Set(ctx context.Context, tenantID, product string, enabled bool) (bool, error) {
	var prev bool
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		// A tenant made by a build older than the product has no row yet: create it disabled.
		if _, err := tx.Exec(ctx, `INSERT INTO tenant_products (tenant_id, product, enabled) VALUES ($1, $2, false) ON CONFLICT (tenant_id, product) DO NOTHING`, tenantID, product); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT enabled FROM tenant_products WHERE tenant_id = $1 AND product = $2 FOR UPDATE`, tenantID, product).Scan(&prev); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE tenant_products SET enabled = $3, updated_at = now() WHERE tenant_id = $1 AND product = $2`, tenantID, product, enabled)
		return err
	})
	return prev, err
}
