package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// resource_grants is one table for every content type. These functions are
// the shared SQL; each content type has its own grant repo that passes its
// fixed object type.

const grantCols = `g.id, g.tenant_id, g.object_type, g.object_id, g.relation, g.subject_type, g.user_id, coalesce(u.name, ''), g.created_at`

func queryGrants(ctx context.Context, pool *pgxpool.Pool, where string, args ...any) ([]entity.Grant, error) {
	rows, err := pool.Query(ctx, `SELECT `+grantCols+` FROM resource_grants g LEFT JOIN users u ON u.id = g.user_id WHERE `+where+
		` ORDER BY g.relation = 'owner' DESC, g.subject_type, u.name`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.Grant{}
	for rows.Next() {
		var g entity.Grant
		if err := rows.Scan(&g.ID, &g.TenantID, &g.ObjectType, &g.ObjectID, &g.Relation, &g.SubjectType, &g.UserID, &g.UserName, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func insertGrant(ctx context.Context, tx pgx.Tx, objectType string, g entity.Grant) error {
	_, err := tx.Exec(ctx, `INSERT INTO resource_grants (id, tenant_id, object_type, object_id, relation, subject_type, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, g.ID, g.TenantID, objectType, g.ObjectID, g.Relation, g.SubjectType, g.UserID)
	return err
}

func listObjectGrants(ctx context.Context, pool *pgxpool.Pool, objectType, objectID string) ([]entity.Grant, error) {
	return queryGrants(ctx, pool, `g.object_type = $1 AND g.object_id = $2`, objectType, objectID)
}

func upsertGrant(ctx context.Context, pool *pgxpool.Pool, objectType string, g entity.Grant) (*entity.Grant, error) {
	var prev *entity.Grant
	err := inTx(ctx, pool, func(tx pgx.Tx) error {
		var row pgx.Row
		if g.SubjectType == entity.SubjectUser {
			row = tx.QueryRow(ctx, `SELECT id, relation FROM resource_grants WHERE object_type = $1 AND object_id = $2 AND subject_type = 'user' AND user_id = $3 FOR UPDATE`,
				objectType, g.ObjectID, g.UserID)
		} else {
			row = tx.QueryRow(ctx, `SELECT id, relation FROM resource_grants WHERE object_type = $1 AND object_id = $2 AND subject_type = $3 FOR UPDATE`,
				objectType, g.ObjectID, g.SubjectType)
		}
		old := g
		switch err := row.Scan(&old.ID, &old.Relation); err {
		case nil:
			prev = &old
			_, err := tx.Exec(ctx, `UPDATE resource_grants SET relation = $2 WHERE id = $1`, old.ID, g.Relation)
			return err
		case pgx.ErrNoRows:
			return insertGrant(ctx, tx, objectType, g)
		default:
			return err
		}
	})
	return prev, err
}

func deleteGrant(ctx context.Context, pool *pgxpool.Pool, objectType, id string) error {
	_, err := pool.Exec(ctx, `DELETE FROM resource_grants WHERE object_type = $1 AND id = $2`, objectType, id)
	return mapErr(err)
}

func restoreGrant(ctx context.Context, pool *pgxpool.Pool, objectType string, g entity.Grant) error {
	_, err := pool.Exec(ctx, `INSERT INTO resource_grants (id, tenant_id, object_type, object_id, relation, subject_type, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO UPDATE SET relation = EXCLUDED.relation`,
		g.ID, g.TenantID, objectType, g.ObjectID, g.Relation, g.SubjectType, g.UserID)
	return mapErr(err)
}

// deleteObjectGrants runs inside a content row delete.
func deleteObjectGrants(ctx context.Context, tx pgx.Tx, objectType, objectID string) error {
	_, err := tx.Exec(ctx, `DELETE FROM resource_grants WHERE object_type = $1 AND object_id = $2`, objectType, objectID)
	return err
}
