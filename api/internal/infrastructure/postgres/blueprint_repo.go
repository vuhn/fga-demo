package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type BlueprintRepo struct{ pool *pgxpool.Pool }

var _ gateway.BlueprintGateway = (*BlueprintRepo)(nil)

func NewBlueprintRepo(pool *pgxpool.Pool) *BlueprintRepo { return &BlueprintRepo{pool: pool} }

func (r *BlueprintRepo) Create(ctx context.Context, b entity.Blueprint, v entity.BlueprintVersion, owner entity.Grant) error {
	var name, mime *string
	var size *int64
	if b.Source != nil {
		name, mime, size = &b.Source.Name, &b.Source.MimeType, &b.Source.ByteSize
	}
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO blueprints (id, tenant_id, project_id, name, current_version, source_kind, source_name, source_mime, source_size, created_by)
			VALUES ($1, $2, $3, $4, 1, $5, $6, $7, $8, $9)`,
			b.ID, b.TenantID, b.ProjectID, b.Name, b.SourceKind, name, mime, size, b.CreatedBy); err != nil {
			return err
		}
		v.Version = 1
		if err := insertBlueprintVersion(ctx, tx, v); err != nil {
			return err
		}
		return insertGrant(ctx, tx, entity.ObjectTypeBlueprint, owner)
	})
}

func insertBlueprintVersion(ctx context.Context, tx pgx.Tx, v entity.BlueprintVersion) error {
	notes, err := json.Marshal(nonNil(v.Notes))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO blueprint_versions (blueprint_id, version, kind, message, brief, model, checks, notes, ok, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		v.BlueprintID, v.Version, v.Kind, v.Message, string(v.Brief), string(v.Model), string(jsonOr(v.Checks, "[]")), string(notes), v.OK, v.CreatedBy)
	return err
}

func (r *BlueprintRepo) query(ctx context.Context, where string, args ...any) ([]entity.Blueprint, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, project_id, name, current_version, source_kind, source_name, source_mime, source_size,
		created_by, created_at, updated_at FROM blueprints WHERE `+where+` ORDER BY name`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.Blueprint{}
	for rows.Next() {
		var b entity.Blueprint
		var name, mime *string
		var size *int64
		if err := rows.Scan(&b.ID, &b.TenantID, &b.ProjectID, &b.Name, &b.CurrentVersion, &b.SourceKind, &name, &mime, &size,
			&b.CreatedBy, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		if name != nil {
			b.Source = &entity.BlueprintSource{Name: *name, MimeType: deref(mime), ByteSize: derefInt(size)}
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *BlueprintRepo) Get(ctx context.Context, id string) (entity.Blueprint, error) {
	bs, err := r.query(ctx, `id = $1`, id)
	if err != nil {
		return entity.Blueprint{}, err
	}
	if len(bs) == 0 {
		return entity.Blueprint{}, entity.ErrNotFound
	}
	return bs[0], nil
}

func (r *BlueprintRepo) ListByIDs(ctx context.Context, tenantID string, ids []string, projectID string) ([]entity.Blueprint, error) {
	return r.query(ctx, `tenant_id = $1 AND id = ANY($2::uuid[]) AND ($3 = '' OR project_id::text = $3)`, tenantID, ids, projectID)
}

func (r *BlueprintRepo) Rename(ctx context.Context, id, name string) (entity.Blueprint, error) {
	if _, err := r.pool.Exec(ctx, `UPDATE blueprints SET name = $2, updated_at = now() WHERE id = $1`, id, name); err != nil {
		return entity.Blueprint{}, mapErr(err)
	}
	return r.Get(ctx, id)
}

func (r *BlueprintRepo) Delete(ctx context.Context, id string) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := deleteObjectGrants(ctx, tx, entity.ObjectTypeBlueprint, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM blueprints WHERE id = $1`, id) // versions cascade
		return err
	})
}

func (r *BlueprintRepo) GetVersion(ctx context.Context, id string, version int) (entity.BlueprintVersion, error) {
	var v entity.BlueprintVersion
	var brief, model, checks, notes []byte
	err := r.pool.QueryRow(ctx, `SELECT blueprint_id, version, kind, message, brief, model, checks, notes, ok, created_by, created_at
		FROM blueprint_versions WHERE blueprint_id = $1 AND version = $2`, id, version).
		Scan(&v.BlueprintID, &v.Version, &v.Kind, &v.Message, &brief, &model, &checks, &notes, &v.OK, &v.CreatedBy, &v.CreatedAt)
	if err != nil {
		return entity.BlueprintVersion{}, mapErr(err)
	}
	v.Brief, v.Model, v.Checks = brief, model, checks
	if err := json.Unmarshal(notes, &v.Notes); err != nil {
		return entity.BlueprintVersion{}, err
	}
	v.Notes = nonNil(v.Notes)
	return v, nil
}

func (r *BlueprintRepo) ListVersions(ctx context.Context, id string) ([]entity.BlueprintVersion, error) {
	rows, err := r.pool.Query(ctx, `SELECT blueprint_id, version, kind, message, ok, created_by, created_at
		FROM blueprint_versions WHERE blueprint_id = $1 ORDER BY version DESC`, id)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.BlueprintVersion{}
	for rows.Next() {
		var v entity.BlueprintVersion
		if err := rows.Scan(&v.BlueprintID, &v.Version, &v.Kind, &v.Message, &v.OK, &v.CreatedBy, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// AppendVersion moves current_version from base to base+1 only while it is
// still base, then inserts the row, in one transaction.
func (r *BlueprintRepo) AppendVersion(ctx context.Context, v entity.BlueprintVersion, base int) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE blueprints SET current_version = $2 + 1, updated_at = now() WHERE id = $1 AND current_version = $2`, v.BlueprintID, base)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return entity.Conflict("blueprint changed since version %d; reload and try again", base)
		}
		v.Version = base + 1
		return insertBlueprintVersion(ctx, tx, v)
	})
}

func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

func jsonOr(raw json.RawMessage, fallback string) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(fallback)
	}
	return raw
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(n *int64) int64 {
	if n == nil {
		return 0
	}
	return *n
}
