package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type ProjectRoleRepo struct{ pool *pgxpool.Pool }

var _ gateway.ProjectRoleGateway = (*ProjectRoleRepo)(nil)

func NewProjectRoleRepo(pool *pgxpool.Pool) *ProjectRoleRepo { return &ProjectRoleRepo{pool: pool} }

func (r *ProjectRoleRepo) List(ctx context.Context, projectID string) ([]entity.ProjectRole, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, project_id, name, affiliation, created_at FROM project_roles WHERE project_id = $1 ORDER BY affiliation, name`, projectID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var out []entity.ProjectRole
	for rows.Next() {
		role, err := scanProjectRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	if err := rows.Err(); err != nil {
		return nil, mapErr(err)
	}
	if out == nil {
		out = []entity.ProjectRole{}
	}
	return r.hydrate(ctx, projectID, out)
}

func (r *ProjectRoleRepo) Get(ctx context.Context, projectID, roleID string) (entity.ProjectRole, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, project_id, name, affiliation, created_at FROM project_roles WHERE project_id = $1 AND id = $2`, projectID, roleID)
	if err != nil {
		return entity.ProjectRole{}, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return entity.ProjectRole{}, mapErr(err)
		}
		return entity.ProjectRole{}, entity.ErrNotFound
	}
	role, err := scanProjectRole(rows)
	if err != nil {
		return entity.ProjectRole{}, err
	}
	roles, err := r.hydrate(ctx, projectID, []entity.ProjectRole{role})
	if err != nil {
		return entity.ProjectRole{}, err
	}
	return roles[0], nil
}

func scanProjectRole(rows pgx.Rows) (entity.ProjectRole, error) {
	var role entity.ProjectRole
	err := rows.Scan(&role.ID, &role.TenantID, &role.ProjectID, &role.Name, &role.Affiliation, &role.CreatedAt)
	return role, mapErr(err)
}

func (r *ProjectRoleRepo) hydrate(ctx context.Context, projectID string, roles []entity.ProjectRole) ([]entity.ProjectRole, error) {
	byID := make(map[string]*entity.ProjectRole, len(roles))
	for i := range roles {
		roles[i].Permissions = []string{}
		roles[i].Assignees = []entity.RoleAssignee{}
		byID[roles[i].ID] = &roles[i]
	}
	perms, err := r.pool.Query(ctx, `SELECT rp.role_id, rp.permission
		FROM project_role_permissions rp JOIN project_roles r ON r.id = rp.role_id
		WHERE r.project_id = $1`, projectID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer perms.Close()
	held := map[string]map[string]bool{}
	for perms.Next() {
		var roleID, permission string
		if err := perms.Scan(&roleID, &permission); err != nil {
			return nil, err
		}
		if held[roleID] == nil {
			held[roleID] = map[string]bool{}
		}
		held[roleID][permission] = true
	}
	if err := perms.Err(); err != nil {
		return nil, err
	}
	for id, role := range byID {
		for _, p := range entity.CustomPermissions {
			if held[id][p] {
				role.Permissions = append(role.Permissions, p)
			}
		}
	}

	assignees, err := r.pool.Query(ctx, `SELECT m.project_role_id, u.id, u.email, u.name
		FROM project_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.project_id = $1 AND m.project_role_id IS NOT NULL
		ORDER BY u.name`, projectID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer assignees.Close()
	for assignees.Next() {
		var roleID string
		var a entity.RoleAssignee
		if err := assignees.Scan(&roleID, &a.UserID, &a.Email, &a.Name); err != nil {
			return nil, err
		}
		if role := byID[roleID]; role != nil {
			role.Assignees = append(role.Assignees, a)
		}
	}
	return roles, assignees.Err()
}

func (r *ProjectRoleRepo) Insert(ctx context.Context, role entity.ProjectRole) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO project_roles (id, tenant_id, project_id, name, affiliation) VALUES ($1, $2, $3, $4, $5)`, role.ID, role.TenantID, role.ProjectID, role.Name, role.Affiliation)
	return mapErr(err)
}

func (r *ProjectRoleRepo) Delete(ctx context.Context, projectID, roleID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM project_roles WHERE project_id = $1 AND id = $2`, projectID, roleID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return entity.ErrNotFound
	}
	return nil
}

func (r *ProjectRoleRepo) InsertPermissions(ctx context.Context, roleID string, permissions []string) error {
	if len(permissions) == 0 {
		return nil
	}
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		for _, p := range permissions {
			if _, err := tx.Exec(ctx, `INSERT INTO project_role_permissions (role_id, permission) VALUES ($1, $2)`, roleID, p); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ProjectRoleRepo) DeletePermissions(ctx context.Context, roleID string, permissions []string) error {
	if len(permissions) == 0 {
		return nil
	}
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		for _, p := range permissions {
			if _, err := tx.Exec(ctx, `DELETE FROM project_role_permissions WHERE role_id = $1 AND permission = $2`, roleID, p); err != nil {
				return err
			}
		}
		return nil
	})
}
