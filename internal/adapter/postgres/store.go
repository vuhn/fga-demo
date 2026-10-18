package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/internal/domain"
	"github.com/vuhn/fga-demo/internal/usecase"
)

type Store struct{ pool *pgxpool.Pool }

var _ usecase.Store = (*Store)(nil)

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Table names come from this fixed map, never from input.
var tables = map[string]string{"folder": "folders", "album": "albums", "document": "documents", "photo": "photos"}

func table(objectType string) string {
	t, ok := tables[objectType]
	if !ok {
		panic("postgres: unknown object type " + objectType)
	}
	return t
}

// ---- Users ----

const userCols = `u.id, u.email, u.name, u.password_hash, u.created_at, EXISTS (SELECT 1 FROM platform_admins pa WHERE pa.user_id = u.id)`

func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt, &u.IsSystemAdmin)
	return u, mapErr(err)
}

func (s *Store) CreateUser(ctx context.Context, u domain.User) error {
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, $3, $4)`,
			u.ID, u.Email, u.Name, u.PasswordHash); err != nil {
			return err
		}
		if u.IsSystemAdmin {
			_, err := tx.Exec(ctx, `INSERT INTO platform_admins (user_id) VALUES ($1)`, u.ID)
			return err
		}
		return nil
	})
}

func (s *Store) GetUser(ctx context.Context, id string) (domain.User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users u WHERE u.id = $1`, id))
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users u WHERE lower(u.email) = lower($1)`, email))
}

func (s *Store) listUsers(ctx context.Context, q string, args ...any) ([]domain.User, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) ListUsers(ctx context.Context) ([]domain.User, error) {
	return s.listUsers(ctx, `SELECT `+userCols+` FROM users u ORDER BY u.name`)
}

func (s *Store) ListPlatformAdmins(ctx context.Context) ([]domain.User, error) {
	return s.listUsers(ctx, `SELECT `+userCols+` FROM users u JOIN platform_admins p ON p.user_id = u.id ORDER BY u.name`)
}

func (s *Store) UpdateUserName(ctx context.Context, id, name string) (domain.User, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET name = $2 WHERE id = $1`, id, name)
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.User{}, domain.ErrNotFound
	}
	return s.GetUser(ctx, id)
}

// ---- Tenants and products ----

func (s *Store) CreateTenant(ctx context.Context, t domain.Tenant) error {
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, slug, name) VALUES ($1, $2, $3)`, t.ID, t.Slug, t.Name); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO tenant_products (tenant_id, product, enabled) VALUES ($1, 'documents', false), ($1, 'photos', false)`, t.ID)
		return err
	})
}

func (s *Store) queryTenants(ctx context.Context, q string, args ...any) ([]domain.Tenant, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domain.Tenant{}
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Products, err = s.ListProducts(ctx, out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) GetTenant(ctx context.Context, id string) (domain.Tenant, error) {
	ts, err := s.queryTenants(ctx, `SELECT id, slug, name, created_at FROM tenants WHERE id = $1`, id)
	if err != nil {
		return domain.Tenant{}, err
	}
	if len(ts) == 0 {
		return domain.Tenant{}, domain.ErrNotFound
	}
	return ts[0], nil
}

func (s *Store) ListTenants(ctx context.Context) ([]domain.Tenant, error) {
	return s.queryTenants(ctx, `SELECT id, slug, name, created_at FROM tenants ORDER BY name`)
}

func (s *Store) TenantsByIDs(ctx context.Context, ids []string) ([]domain.Tenant, error) {
	return s.queryTenants(ctx, `SELECT id, slug, name, created_at FROM tenants WHERE id = ANY($1::uuid[]) ORDER BY name`, ids)
}

func (s *Store) UpdateTenantName(ctx context.Context, id, name string) (domain.Tenant, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE tenants SET name = $2 WHERE id = $1`, id, name)
	if err != nil {
		return domain.Tenant{}, mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Tenant{}, domain.ErrNotFound
	}
	return s.GetTenant(ctx, id)
}

func (s *Store) ListProducts(ctx context.Context, tenantID string) ([]domain.Product, error) {
	rows, err := s.pool.Query(ctx, `SELECT product, enabled, updated_at FROM tenant_products WHERE tenant_id = $1 ORDER BY product`, tenantID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domain.Product{}
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.Name, &p.Enabled, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) SetProduct(ctx context.Context, tenantID, product string, enabled bool) (bool, error) {
	var prev bool
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT enabled FROM tenant_products WHERE tenant_id = $1 AND product = $2 FOR UPDATE`, tenantID, product).Scan(&prev); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE tenant_products SET enabled = $3, updated_at = now() WHERE tenant_id = $1 AND product = $2`, tenantID, product, enabled)
		return err
	})
	return prev, err
}

// ---- Memberships ----

const memberCols = `m.tenant_id, m.user_id, m.affiliation, m.role, u.email, u.name`

func (s *Store) queryMembers(ctx context.Context, q string, args ...any) ([]domain.Membership, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domain.Membership{}
	for rows.Next() {
		var m domain.Membership
		if err := rows.Scan(&m.TenantID, &m.UserID, &m.Affiliation, &m.Role, &m.Email, &m.Name); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetMembership(ctx context.Context, tenantID, userID string) (domain.Membership, error) {
	ms, err := s.queryMembers(ctx, `SELECT `+memberCols+` FROM memberships m JOIN users u ON u.id = m.user_id WHERE m.tenant_id = $1 AND m.user_id = $2`, tenantID, userID)
	if err != nil {
		return domain.Membership{}, err
	}
	if len(ms) == 0 {
		return domain.Membership{}, domain.ErrNotFound
	}
	return ms[0], nil
}

func (s *Store) ListMembers(ctx context.Context, tenantID, affiliation string) ([]domain.Membership, error) {
	return s.queryMembers(ctx, `SELECT `+memberCols+` FROM memberships m JOIN users u ON u.id = m.user_id
		WHERE m.tenant_id = $1 AND ($2 = '' OR m.affiliation = $2) ORDER BY m.affiliation, u.name`, tenantID, affiliation)
}

func (s *Store) MembershipsForUser(ctx context.Context, userID string) ([]domain.Membership, error) {
	return s.queryMembers(ctx, `SELECT `+memberCols+` FROM memberships m JOIN users u ON u.id = m.user_id WHERE m.user_id = $1`, userID)
}

func (s *Store) InsertMembership(ctx context.Context, m domain.Membership) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO memberships (tenant_id, user_id, affiliation, role) VALUES ($1, $2, $3, $4)`,
		m.TenantID, m.UserID, m.Affiliation, m.Role)
	return mapErr(err)
}

func (s *Store) UpdateRole(ctx context.Context, tenantID, userID, role string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE memberships SET role = $3 WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID, role)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) CountInternalAdmins(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM memberships WHERE tenant_id = $1 AND role = 'internal_admin'`, tenantID).Scan(&n)
	return n, mapErr(err)
}

func (s *Store) RemoveMember(ctx context.Context, tenantID, userID string) error {
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM resource_grants WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM memberships WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID)
		return err
	})
}

// ---- Containers ----

func insertGrant(ctx context.Context, tx pgx.Tx, g domain.Grant) error {
	_, err := tx.Exec(ctx, `INSERT INTO resource_grants (id, tenant_id, object_type, object_id, relation, subject_type, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, g.ID, g.TenantID, g.ObjectType, g.ObjectID, g.Relation, g.SubjectType, g.UserID)
	return err
}

func (s *Store) CreateContainer(ctx context.Context, kind domain.Kind, c domain.Container, owner domain.Grant) error {
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO `+table(kind.Container)+` (id, tenant_id, parent_id, name, is_private, created_by) VALUES ($1, $2, $3, $4, $5, $6)`,
			c.ID, c.TenantID, c.ParentID, c.Name, c.IsPrivate, c.CreatedBy); err != nil {
			return err
		}
		return insertGrant(ctx, tx, owner)
	})
}

func (s *Store) queryContainers(ctx context.Context, kind domain.Kind, where string, args ...any) ([]domain.Container, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, tenant_id, parent_id, name, is_private, created_by, created_at FROM `+table(kind.Container)+` WHERE `+where+` ORDER BY name`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domain.Container{}
	for rows.Next() {
		var c domain.Container
		if err := rows.Scan(&c.ID, &c.TenantID, &c.ParentID, &c.Name, &c.IsPrivate, &c.CreatedBy, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetContainer(ctx context.Context, kind domain.Kind, id string) (domain.Container, error) {
	cs, err := s.queryContainers(ctx, kind, `id = $1`, id)
	if err != nil {
		return domain.Container{}, err
	}
	if len(cs) == 0 {
		return domain.Container{}, domain.ErrNotFound
	}
	return cs[0], nil
}

func (s *Store) ContainersByIDs(ctx context.Context, kind domain.Kind, tenantID string, ids []string) ([]domain.Container, error) {
	return s.queryContainers(ctx, kind, `tenant_id = $1 AND id = ANY($2::uuid[])`, tenantID, ids)
}

func (s *Store) RenameContainer(ctx context.Context, kind domain.Kind, id, name string) (domain.Container, error) {
	if _, err := s.pool.Exec(ctx, `UPDATE `+table(kind.Container)+` SET name = $2 WHERE id = $1`, id, name); err != nil {
		return domain.Container{}, mapErr(err)
	}
	return s.GetContainer(ctx, kind, id)
}

func (s *Store) SetContainerPrivate(ctx context.Context, kind domain.Kind, id string, isPrivate bool) (bool, error) {
	var prev bool
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT is_private FROM `+table(kind.Container)+` WHERE id = $1 FOR UPDATE`, id).Scan(&prev); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE `+table(kind.Container)+` SET is_private = $2 WHERE id = $1`, id, isPrivate)
		return err
	})
	return prev, err
}

func (s *Store) ContainerHasChildren(ctx context.Context, kind domain.Kind, id string) (bool, error) {
	var has bool
	q := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE parent_id = $1) OR EXISTS (SELECT 1 FROM %s WHERE %s_id = $1)`,
		table(kind.Container), table(kind.Item), kind.ContainerRef)
	err := s.pool.QueryRow(ctx, q, id).Scan(&has)
	return has, mapErr(err)
}

// ---- Items ----

func (s *Store) CreateItem(ctx context.Context, kind domain.Kind, it domain.Item, owner domain.Grant) error {
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := fmt.Sprintf(`INSERT INTO %s (id, tenant_id, %s_id, name, mime_type, byte_size, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			table(kind.Item), kind.ContainerRef)
		if _, err := tx.Exec(ctx, q, it.ID, it.TenantID, it.ContainerID, it.Name, it.MimeType, it.ByteSize, it.CreatedBy); err != nil {
			return err
		}
		return insertGrant(ctx, tx, owner)
	})
}

func (s *Store) queryItems(ctx context.Context, kind domain.Kind, where string, args ...any) ([]domain.Item, error) {
	q := fmt.Sprintf(`SELECT id, tenant_id, %s_id, name, mime_type, byte_size, created_by, created_at FROM %s WHERE %s ORDER BY name`,
		kind.ContainerRef, table(kind.Item), where)
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domain.Item{}
	for rows.Next() {
		var it domain.Item
		if err := rows.Scan(&it.ID, &it.TenantID, &it.ContainerID, &it.Name, &it.MimeType, &it.ByteSize, &it.CreatedBy, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) GetItem(ctx context.Context, kind domain.Kind, id string) (domain.Item, error) {
	its, err := s.queryItems(ctx, kind, `id = $1`, id)
	if err != nil {
		return domain.Item{}, err
	}
	if len(its) == 0 {
		return domain.Item{}, domain.ErrNotFound
	}
	return its[0], nil
}

func (s *Store) ItemsByIDs(ctx context.Context, kind domain.Kind, tenantID string, ids []string, containerID string) ([]domain.Item, error) {
	if containerID == "" {
		return s.queryItems(ctx, kind, `tenant_id = $1 AND id = ANY($2::uuid[])`, tenantID, ids)
	}
	return s.queryItems(ctx, kind, fmt.Sprintf(`tenant_id = $1 AND id = ANY($2::uuid[]) AND %s_id = $3`, kind.ContainerRef), tenantID, ids, containerID)
}

func (s *Store) RenameItem(ctx context.Context, kind domain.Kind, id, name string) (domain.Item, error) {
	if _, err := s.pool.Exec(ctx, `UPDATE `+table(kind.Item)+` SET name = $2 WHERE id = $1`, id, name); err != nil {
		return domain.Item{}, mapErr(err)
	}
	return s.GetItem(ctx, kind, id)
}

func (s *Store) DeleteObject(ctx context.Context, objectType, id string) error {
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM resource_grants WHERE object_type = $1 AND object_id = $2`, objectType, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM `+table(objectType)+` WHERE id = $1`, id)
		return err
	})
}

// ---- Grants ----

const grantCols = `g.id, g.tenant_id, g.object_type, g.object_id, g.relation, g.subject_type, g.user_id, coalesce(u.name, ''), g.created_at`

func (s *Store) queryGrants(ctx context.Context, where string, args ...any) ([]domain.Grant, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+grantCols+` FROM resource_grants g LEFT JOIN users u ON u.id = g.user_id WHERE `+where+
		` ORDER BY g.relation = 'owner' DESC, g.subject_type, u.name`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domain.Grant{}
	for rows.Next() {
		var g domain.Grant
		if err := rows.Scan(&g.ID, &g.TenantID, &g.ObjectType, &g.ObjectID, &g.Relation, &g.SubjectType, &g.UserID, &g.UserName, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) ListGrants(ctx context.Context, objectType, objectID string) ([]domain.Grant, error) {
	return s.queryGrants(ctx, `g.object_type = $1 AND g.object_id = $2`, objectType, objectID)
}

func (s *Store) GrantsForUser(ctx context.Context, tenantID, userID string) ([]domain.Grant, error) {
	return s.queryGrants(ctx, `g.tenant_id = $1 AND g.user_id = $2`, tenantID, userID)
}

func (s *Store) UpsertGrant(ctx context.Context, g domain.Grant) (*domain.Grant, error) {
	var prev *domain.Grant
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		var row pgx.Row
		if g.SubjectType == domain.SubjectUser {
			row = tx.QueryRow(ctx, `SELECT id, relation FROM resource_grants WHERE object_type = $1 AND object_id = $2 AND subject_type = 'user' AND user_id = $3 FOR UPDATE`,
				g.ObjectType, g.ObjectID, g.UserID)
		} else {
			row = tx.QueryRow(ctx, `SELECT id, relation FROM resource_grants WHERE object_type = $1 AND object_id = $2 AND subject_type = $3 FOR UPDATE`,
				g.ObjectType, g.ObjectID, g.SubjectType)
		}
		old := g
		switch err := row.Scan(&old.ID, &old.Relation); err {
		case nil:
			prev = &old
			_, err := tx.Exec(ctx, `UPDATE resource_grants SET relation = $2 WHERE id = $1`, old.ID, g.Relation)
			return err
		case pgx.ErrNoRows:
			return insertGrant(ctx, tx, g)
		default:
			return err
		}
	})
	return prev, err
}

func (s *Store) DeleteGrant(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM resource_grants WHERE id = $1`, id)
	return mapErr(err)
}

// RestoreGrant puts a grant row back exactly, inserting or updating by id.
func (s *Store) RestoreGrant(ctx context.Context, g domain.Grant) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO resource_grants (id, tenant_id, object_type, object_id, relation, subject_type, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO UPDATE SET relation = EXCLUDED.relation`,
		g.ID, g.TenantID, g.ObjectType, g.ObjectID, g.Relation, g.SubjectType, g.UserID)
	return mapErr(err)
}
