package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type MembershipRepo struct{ pool *pgxpool.Pool }

var _ gateway.MembershipGateway = (*MembershipRepo)(nil)

func NewMembershipRepo(pool *pgxpool.Pool) *MembershipRepo { return &MembershipRepo{pool: pool} }

// affiliationExpr is the single derivation: internal iff the user's company is the tenant's own.
const affiliationExpr = `CASE WHEN c.tenant_id = m.tenant_id THEN 'internal' ELSE 'external' END`

const memberCols = `m.tenant_id, m.user_id, ` + affiliationExpr + `,
	CASE WHEN m.tenant_role_id IS NULL THEN m.role ELSE m.tenant_role_id::text END,
	COALESCE(cr.name, ''), COALESCE(m.tenant_role_id::text, ''), u.email, u.name, c.id, c.name`
const memberFrom = `tenant_members m JOIN users u ON u.id = m.user_id JOIN companies c ON c.id = u.company_id
	LEFT JOIN tenant_roles cr ON cr.id = m.tenant_role_id`

func (r *MembershipRepo) query(ctx context.Context, q string, args ...any) ([]entity.Membership, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.Membership{}
	for rows.Next() {
		var m entity.Membership
		if err := rows.Scan(&m.TenantID, &m.UserID, &m.Affiliation, &m.Role, &m.RoleName, &m.CustomRoleID, &m.Email, &m.Name, &m.CompanyID, &m.CompanyName); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MembershipRepo) Get(ctx context.Context, tenantID, userID string) (entity.Membership, error) {
	ms, err := r.query(ctx, `SELECT `+memberCols+` FROM `+memberFrom+` WHERE m.tenant_id = $1 AND m.user_id = $2`, tenantID, userID)
	if err != nil {
		return entity.Membership{}, err
	}
	if len(ms) == 0 {
		return entity.Membership{}, entity.ErrNotFound
	}
	return ms[0], nil
}

func (r *MembershipRepo) List(ctx context.Context, tenantID, affiliation string) ([]entity.Membership, error) {
	return r.query(ctx, `SELECT `+memberCols+` FROM `+memberFrom+`
		WHERE m.tenant_id = $1 AND ($2 = '' OR `+affiliationExpr+` = $2) ORDER BY `+affiliationExpr+`, u.name`, tenantID, affiliation)
}

func (r *MembershipRepo) ListForUser(ctx context.Context, userID string) ([]entity.Membership, error) {
	return r.query(ctx, `SELECT `+memberCols+` FROM `+memberFrom+` WHERE m.user_id = $1`, userID)
}

func (r *MembershipRepo) Insert(ctx context.Context, m entity.Membership) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO tenant_members (tenant_id, user_id, role, tenant_role_id)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid)`,
		m.TenantID, m.UserID, m.Role, m.CustomRoleID)
	return mapErr(err)
}

func (r *MembershipRepo) UpdateRole(ctx context.Context, tenantID, userID, role, customRoleID string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE tenant_members SET role = $3, tenant_role_id = NULLIF($4, '')::uuid WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID, role, customRoleID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return entity.ErrNotFound
	}
	return nil
}

func (r *MembershipRepo) CountInternalAdmins(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM tenant_members WHERE tenant_id = $1 AND role = 'internal_admin'`, tenantID).Scan(&n)
	return n, mapErr(err)
}

func (r *MembershipRepo) ListUserGrants(ctx context.Context, tenantID, userID string) ([]entity.Grant, error) {
	return queryGrants(ctx, r.pool, `g.tenant_id = $1 AND g.user_id = $2`, tenantID, userID)
}

func (r *MembershipRepo) Delete(ctx context.Context, tenantID, userID string) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM resource_grants WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM tenant_members WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID)
		return err
	})
}
