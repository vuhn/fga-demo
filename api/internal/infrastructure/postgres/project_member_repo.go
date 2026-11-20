package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type ProjectMemberRepo struct{ pool *pgxpool.Pool }

var _ gateway.ProjectMemberGateway = (*ProjectMemberRepo)(nil)

func NewProjectMemberRepo(pool *pgxpool.Pool) *ProjectMemberRepo {
	return &ProjectMemberRepo{pool: pool}
}

const projectMemberCols = `pm.project_id, pm.tenant_id, pm.user_id,
	CASE WHEN c.tenant_id = pm.tenant_id THEN 'internal' ELSE 'external' END,
	CASE WHEN pm.project_role_id IS NULL THEN pm.role ELSE pm.project_role_id::text END,
	COALESCE(pr.name, ''), COALESCE(pm.project_role_id::text, ''), u.email, u.name, c.id, c.name`
const projectMemberFrom = `project_members pm JOIN users u ON u.id = pm.user_id JOIN companies c ON c.id = u.company_id
	LEFT JOIN project_roles pr ON pr.id = pm.project_role_id`

func (r *ProjectMemberRepo) query(ctx context.Context, where string, args ...any) ([]model.ProjectMember, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+projectMemberCols+` FROM `+projectMemberFrom+` WHERE `+where+` ORDER BY u.name`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []model.ProjectMember{}
	for rows.Next() {
		var m model.ProjectMember
		if err := rows.Scan(&m.ProjectID, &m.TenantID, &m.UserID, &m.Affiliation, &m.Role, &m.RoleName, &m.ProjectRoleID, &m.Email, &m.Name, &m.CompanyID, &m.CompanyName); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *ProjectMemberRepo) Get(ctx context.Context, projectID, userID string) (model.ProjectMember, error) {
	ms, err := r.query(ctx, `pm.project_id = $1 AND pm.user_id = $2`, projectID, userID)
	if err != nil {
		return model.ProjectMember{}, err
	}
	if len(ms) == 0 {
		return model.ProjectMember{}, model.ErrNotFound
	}
	return ms[0], nil
}

func (r *ProjectMemberRepo) List(ctx context.Context, projectID string) ([]model.ProjectMember, error) {
	return r.query(ctx, `pm.project_id = $1`, projectID)
}

func (r *ProjectMemberRepo) ListForUser(ctx context.Context, tenantID, userID string) ([]model.ProjectMember, error) {
	return r.query(ctx, `pm.tenant_id = $1 AND pm.user_id = $2`, tenantID, userID)
}

func (r *ProjectMemberRepo) Insert(ctx context.Context, m model.ProjectMember) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO project_members (project_id, tenant_id, user_id, role, project_role_id)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid)`, m.ProjectID, m.TenantID, m.UserID, m.Role, m.ProjectRoleID)
	return mapErr(err)
}

func (r *ProjectMemberRepo) UpdateRole(ctx context.Context, p model.UpdateProjectMemberRoleParams) error {
	tag, err := r.pool.Exec(ctx, `UPDATE project_members SET role = $3, project_role_id = NULLIF($4, '')::uuid
		WHERE project_id = $1 AND user_id = $2`, p.ProjectID, p.UserID, p.Role, p.ProjectRoleID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *ProjectMemberRepo) Delete(ctx context.Context, projectID, userID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`, projectID, userID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
