package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type RoleRepo struct{ pool *pgxpool.Pool }

var _ gateway.RoleGateway = (*RoleRepo)(nil)

func NewRoleRepo(pool *pgxpool.Pool) *RoleRepo { return &RoleRepo{pool: pool} }

func (r *RoleRepo) List(ctx context.Context, tenantID string) ([]model.Role, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, name, affiliation, created_at FROM tenant_roles WHERE tenant_id = $1 ORDER BY affiliation, name`, tenantID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var out []model.Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	if err := rows.Err(); err != nil {
		return nil, mapErr(err)
	}
	if out == nil {
		out = []model.Role{}
	}
	return r.hydrate(ctx, tenantID, out)
}

func (r *RoleRepo) Get(ctx context.Context, tenantID, roleID string) (model.Role, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, name, affiliation, created_at FROM tenant_roles WHERE tenant_id = $1 AND id = $2`, tenantID, roleID)
	if err != nil {
		return model.Role{}, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return model.Role{}, mapErr(err)
		}
		return model.Role{}, model.ErrNotFound
	}
	role, err := scanRole(rows)
	if err != nil {
		return model.Role{}, err
	}
	roles, err := r.hydrate(ctx, tenantID, []model.Role{role})
	if err != nil {
		return model.Role{}, err
	}
	return roles[0], nil
}

func scanRole(rows pgx.Rows) (model.Role, error) {
	var role model.Role
	err := rows.Scan(&role.ID, &role.TenantID, &role.Name, &role.Affiliation, &role.CreatedAt)
	return role, mapErr(err)
}

func (r *RoleRepo) hydrate(ctx context.Context, tenantID string, roles []model.Role) ([]model.Role, error) {
	byID := make(map[string]*model.Role, len(roles))
	for i := range roles {
		roles[i].Permissions = []string{}
		roles[i].Assignees = []model.RoleAssignee{}
		byID[roles[i].ID] = &roles[i]
	}
	perms, err := r.pool.Query(ctx, `SELECT rp.role_id, rp.permission
		FROM tenant_role_permissions rp JOIN tenant_roles r ON r.id = rp.role_id
		WHERE r.tenant_id = $1`, tenantID)
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
		for _, p := range model.CustomPermissions {
			if held[id][p] {
				role.Permissions = append(role.Permissions, p)
			}
		}
	}

	assignees, err := r.pool.Query(ctx, `SELECT m.tenant_role_id, u.id, u.email, u.name
		FROM tenant_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.tenant_id = $1 AND m.tenant_role_id IS NOT NULL
		ORDER BY u.name`, tenantID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer assignees.Close()
	for assignees.Next() {
		var roleID string
		var a model.RoleAssignee
		if err := assignees.Scan(&roleID, &a.UserID, &a.Email, &a.Name); err != nil {
			return nil, err
		}
		if role := byID[roleID]; role != nil {
			role.Assignees = append(role.Assignees, a)
		}
	}
	return roles, assignees.Err()
}

func (r *RoleRepo) Insert(ctx context.Context, role model.Role) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO tenant_roles (id, tenant_id, name, affiliation) VALUES ($1, $2, $3, $4)`, role.ID, role.TenantID, role.Name, role.Affiliation)
	return mapErr(err)
}

func (r *RoleRepo) UpdateName(ctx context.Context, tenantID, roleID, name string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE tenant_roles SET name = $3 WHERE tenant_id = $1 AND id = $2`, tenantID, roleID, name)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *RoleRepo) Delete(ctx context.Context, tenantID, roleID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tenant_roles WHERE tenant_id = $1 AND id = $2`, tenantID, roleID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *RoleRepo) InsertPermissions(ctx context.Context, roleID string, permissions []string) error {
	if len(permissions) == 0 {
		return nil
	}
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		for _, p := range permissions {
			if _, err := tx.Exec(ctx, `INSERT INTO tenant_role_permissions (role_id, permission) VALUES ($1, $2)`, roleID, p); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RoleRepo) DeletePermissions(ctx context.Context, roleID string, permissions []string) error {
	if len(permissions) == 0 {
		return nil
	}
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		for _, p := range permissions {
			if _, err := tx.Exec(ctx, `DELETE FROM tenant_role_permissions WHERE role_id = $1 AND permission = $2`, roleID, p); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RoleRepo) ListRoleIDsForUser(ctx context.Context, tenantID, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT tenant_role_id FROM tenant_members
		WHERE tenant_id = $1 AND user_id = $2 AND tenant_role_id IS NOT NULL`, tenantID, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
