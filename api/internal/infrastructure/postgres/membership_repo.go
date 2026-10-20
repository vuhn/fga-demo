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

const memberCols = `m.tenant_id, m.user_id, m.affiliation,
	CASE WHEN m.custom_role_id IS NULL THEN m.role ELSE m.custom_role_id::text END,
	COALESCE(cr.name, ''), COALESCE(m.custom_role_id::text, ''), u.email, u.name`
const memberFrom = `memberships m JOIN users u ON u.id = m.user_id LEFT JOIN roles cr ON cr.id = m.custom_role_id`

func (r *MembershipRepo) query(ctx context.Context, q string, args ...any) ([]entity.Membership, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.Membership{}
	for rows.Next() {
		var m entity.Membership
		if err := rows.Scan(&m.TenantID, &m.UserID, &m.Affiliation, &m.Role, &m.RoleName, &m.CustomRoleID, &m.Email, &m.Name); err != nil {
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
		WHERE m.tenant_id = $1 AND ($2 = '' OR m.affiliation = $2) ORDER BY m.affiliation, u.name`, tenantID, affiliation)
}

func (r *MembershipRepo) ListForUser(ctx context.Context, userID string) ([]entity.Membership, error) {
	return r.query(ctx, `SELECT `+memberCols+` FROM `+memberFrom+` WHERE m.user_id = $1`, userID)
}

func (r *MembershipRepo) Insert(ctx context.Context, m entity.Membership) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO memberships (tenant_id, user_id, affiliation, role, custom_role_id)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid)`,
		m.TenantID, m.UserID, m.Affiliation, m.Role, m.CustomRoleID)
	return mapErr(err)
}

func (r *MembershipRepo) UpdateRole(ctx context.Context, tenantID, userID, role, customRoleID string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE memberships SET role = $3, custom_role_id = NULLIF($4, '')::uuid WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID, role, customRoleID)
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
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM memberships WHERE tenant_id = $1 AND role = 'internal_admin'`, tenantID).Scan(&n)
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
		_, err := tx.Exec(ctx, `DELETE FROM memberships WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID)
		return err
	})
}
