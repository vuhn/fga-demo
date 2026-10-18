package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type UserRepo struct{ pool *pgxpool.Pool }

var _ gateway.UserGateway = (*UserRepo)(nil)

func NewUserRepo(pool *pgxpool.Pool) *UserRepo { return &UserRepo{pool: pool} }

const userCols = `u.id, u.email, u.name, u.password_hash, u.created_at, EXISTS (SELECT 1 FROM platform_admins pa WHERE pa.user_id = u.id)`

func scanUser(row pgx.Row) (entity.User, error) {
	var u entity.User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt, &u.IsSystemAdmin)
	return u, mapErr(err)
}

func (r *UserRepo) Create(ctx context.Context, u entity.User) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
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

func (r *UserRepo) Get(ctx context.Context, id string) (entity.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users u WHERE u.id = $1`, id))
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (entity.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users u WHERE lower(u.email) = lower($1)`, email))
}

func (r *UserRepo) query(ctx context.Context, q string, args ...any) ([]entity.User, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []entity.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *UserRepo) List(ctx context.Context) ([]entity.User, error) {
	return r.query(ctx, `SELECT `+userCols+` FROM users u ORDER BY u.name`)
}

func (r *UserRepo) ListPlatformAdmins(ctx context.Context) ([]entity.User, error) {
	return r.query(ctx, `SELECT `+userCols+` FROM users u JOIN platform_admins p ON p.user_id = u.id ORDER BY u.name`)
}

func (r *UserRepo) UpdateName(ctx context.Context, id, name string) (entity.User, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE users SET name = $2 WHERE id = $1`, id, name)
	if err != nil {
		return entity.User{}, mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return entity.User{}, entity.ErrNotFound
	}
	return r.Get(ctx, id)
}
