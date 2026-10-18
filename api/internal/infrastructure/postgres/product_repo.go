package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type ProductRepo struct{ pool *pgxpool.Pool }

var _ gateway.ProductGateway = (*ProductRepo)(nil)

func NewProductRepo(pool *pgxpool.Pool) *ProductRepo { return &ProductRepo{pool: pool} }

func (r *ProductRepo) List(ctx context.Context, tenantID string) ([]entity.Product, error) {
	return listProducts(ctx, r.pool, tenantID)
}

func listProducts(ctx context.Context, pool *pgxpool.Pool, tenantID string) ([]entity.Product, error) {
	rows, err := pool.Query(ctx, `SELECT product, enabled, updated_at FROM tenant_products WHERE tenant_id = $1 ORDER BY product`, tenantID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.Product{}
	for rows.Next() {
		var p entity.Product
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
		if err := tx.QueryRow(ctx, `SELECT enabled FROM tenant_products WHERE tenant_id = $1 AND product = $2 FOR UPDATE`, tenantID, product).Scan(&prev); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE tenant_products SET enabled = $3, updated_at = now() WHERE tenant_id = $1 AND product = $2`, tenantID, product, enabled)
		return err
	})
	return prev, err
}
