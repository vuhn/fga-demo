package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type CompanyRepo struct{ pool *pgxpool.Pool }

var _ gateway.CompanyGateway = (*CompanyRepo)(nil)

func NewCompanyRepo(pool *pgxpool.Pool) *CompanyRepo { return &CompanyRepo{pool: pool} }

const companyCols = `c.id, c.name, c.name_key, COALESCE(c.tax_id, ''), COALESCE(c.created_by_tenant_id::text, ''),
	COALESCE(c.tenant_id::text, ''), c.claimed_at, c.created_at, (SELECT count(*) FROM users u WHERE u.company_id = c.id)`

func (r *CompanyRepo) query(ctx context.Context, where string, args ...any) ([]entity.Company, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+companyCols+` FROM companies c WHERE `+where+` ORDER BY c.name`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.Company{}
	for rows.Next() {
		var c entity.Company
		if err := rows.Scan(&c.ID, &c.Name, &c.NameKey, &c.TaxID, &c.CreatedByTenantID, &c.TenantID, &c.ClaimedAt, &c.CreatedAt, &c.MemberCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CompanyRepo) Create(ctx context.Context, c entity.Company) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO companies (id, name, name_key, tax_id, created_by_tenant_id)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, '')::uuid)`, c.ID, c.Name, c.NameKey, c.TaxID, c.CreatedByTenantID)
	return mapErr(err)
}

func (r *CompanyRepo) Get(ctx context.Context, id string) (entity.Company, error) {
	cs, err := r.query(ctx, `c.id = $1`, id)
	if err != nil {
		return entity.Company{}, err
	}
	if len(cs) == 0 {
		return entity.Company{}, entity.ErrNotFound
	}
	return cs[0], nil
}

func (r *CompanyRepo) GetOwn(ctx context.Context, tenantID string) (entity.Company, error) {
	cs, err := r.query(ctx, `c.tenant_id = $1`, tenantID)
	if err != nil {
		return entity.Company{}, err
	}
	if len(cs) == 0 {
		return entity.Company{}, entity.ErrNotFound
	}
	return cs[0], nil
}

func (r *CompanyRepo) List(ctx context.Context) ([]entity.Company, error) {
	return r.query(ctx, `true`)
}

func (r *CompanyRepo) ListForTenant(ctx context.Context, tenantID string) ([]entity.Company, error) {
	return r.query(ctx, `c.tenant_id = $1 OR c.created_by_tenant_id = $1`, tenantID)
}

func (r *CompanyRepo) CountMembersIn(ctx context.Context, tenantID string, companyIDs []string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM tenant_members m JOIN users u ON u.id = m.user_id
		WHERE m.tenant_id = $1 AND u.company_id = ANY($2::uuid[])`, tenantID, companyIDs).Scan(&n)
	return n, mapErr(err)
}

func (r *CompanyRepo) Merge(ctx context.Context, intoID string, fromIDs []string) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE users SET company_id = $1 WHERE company_id = ANY($2::uuid[])`, intoID, fromIDs); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM companies WHERE id = ANY($1::uuid[]) AND tenant_id IS NULL`, fromIDs)
		return err
	})
}
