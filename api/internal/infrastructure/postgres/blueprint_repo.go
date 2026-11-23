package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type BlueprintRepo struct{ pool *pgxpool.Pool }

var _ gateway.BlueprintGateway = (*BlueprintRepo)(nil)

func NewBlueprintRepo(pool *pgxpool.Pool) *BlueprintRepo { return &BlueprintRepo{pool: pool} }

func (r *BlueprintRepo) Create(ctx context.Context, b model.Blueprint, v model.BlueprintVersion, owner model.Grant) error {
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
		if _, err := insertBlueprintVersion(ctx, tx, v); err != nil {
			return err
		}
		return insertGrant(ctx, tx, model.ObjectTypeBlueprint, owner)
	})
}

// CreateImported is Create plus the original file, in one transaction.
func (r *BlueprintRepo) CreateImported(ctx context.Context, p model.CreateImportedBlueprintParams) error {
	b, v, o := p.Blueprint, p.Version, p.Original
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
		if _, err := insertBlueprintVersion(ctx, tx, v); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO blueprint_originals (blueprint_id, version, name, mime_type, byte_size, sha256, data)
			VALUES ($1, 1, $2, $3, $4, $5, $6)`, b.ID, o.Name, o.MimeType, o.ByteSize, o.SHA256, o.Data); err != nil {
			return err
		}
		return insertGrant(ctx, tx, model.ObjectTypeBlueprint, p.Owner)
	})
}

func (r *BlueprintRepo) GetOriginal(ctx context.Context, id string, version int) (model.BlueprintOriginal, error) {
	o := model.BlueprintOriginal{BlueprintID: id, Version: version}
	err := r.pool.QueryRow(ctx, `SELECT name, mime_type, byte_size, sha256, data FROM blueprint_originals WHERE blueprint_id = $1 AND version = $2`,
		id, version).Scan(&o.Name, &o.MimeType, &o.ByteSize, &o.SHA256, &o.Data)
	if err != nil {
		return model.BlueprintOriginal{}, mapErr(err)
	}
	return o, nil
}

func (r *BlueprintRepo) FindOriginal(ctx context.Context, id string) (model.BlueprintOriginal, error) {
	o := model.BlueprintOriginal{BlueprintID: id}
	err := r.pool.QueryRow(ctx, `SELECT version, name, mime_type, byte_size, sha256, data FROM blueprint_originals
		WHERE blueprint_id = $1 ORDER BY version DESC LIMIT 1`, id).Scan(&o.Version, &o.Name, &o.MimeType, &o.ByteSize, &o.SHA256, &o.Data)
	if err != nil {
		return model.BlueprintOriginal{}, mapErr(err)
	}
	return o, nil
}

func insertBlueprintVersion(ctx context.Context, tx pgx.Tx, v model.BlueprintVersion) (model.BlueprintVersion, error) {
	notes, err := json.Marshal(nonNil(v.Notes))
	if err != nil {
		return model.BlueprintVersion{}, err
	}
	return scanBlueprintVersion(tx.QueryRow(ctx, `INSERT INTO blueprint_versions (blueprint_id, version, kind, message, brief, model, checks, notes, ok, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING `+blueprintVersionColumns,
		v.BlueprintID, v.Version, v.Kind, v.Message, string(v.Brief), string(v.Model), string(jsonOr(v.Checks, "[]")), string(notes), v.OK, v.CreatedBy))
}

const blueprintColumns = `id, tenant_id, project_id, name, current_version, source_kind, source_name, source_mime, source_size,
		created_by, created_at, updated_at`

// scanBlueprint reads a row selected with blueprintColumns (or returned by an UPDATE of them).
func scanBlueprint(row pgx.Row) (model.Blueprint, error) {
	var b model.Blueprint
	var name, mime *string
	var size *int64
	if err := row.Scan(&b.ID, &b.TenantID, &b.ProjectID, &b.Name, &b.CurrentVersion, &b.SourceKind, &name, &mime, &size,
		&b.CreatedBy, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return model.Blueprint{}, err
	}
	if name != nil {
		b.Source = &model.BlueprintSource{Name: *name, MimeType: deref(mime), ByteSize: derefInt(size)}
	}
	return b, nil
}

func (r *BlueprintRepo) query(ctx context.Context, where string, args ...any) ([]model.Blueprint, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+blueprintColumns+` FROM blueprints WHERE `+where+` ORDER BY name`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []model.Blueprint{}
	for rows.Next() {
		b, err := scanBlueprint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *BlueprintRepo) Get(ctx context.Context, id string) (model.Blueprint, error) {
	bs, err := r.query(ctx, `id = $1`, id)
	if err != nil {
		return model.Blueprint{}, err
	}
	if len(bs) == 0 {
		return model.Blueprint{}, model.ErrNotFound
	}
	return bs[0], nil
}

func (r *BlueprintRepo) ListByIDs(ctx context.Context, tenantID string, ids []string, projectID string) ([]model.Blueprint, error) {
	return r.query(ctx, `tenant_id = $1 AND id = ANY($2::uuid[]) AND ($3 = '' OR project_id::text = $3)`, tenantID, ids, projectID)
}

func (r *BlueprintRepo) Rename(ctx context.Context, id, name string) (model.Blueprint, error) {
	if _, err := r.pool.Exec(ctx, `UPDATE blueprints SET name = $2, updated_at = now() WHERE id = $1`, id, name); err != nil {
		return model.Blueprint{}, mapErr(err)
	}
	return r.Get(ctx, id)
}

func (r *BlueprintRepo) Delete(ctx context.Context, id string) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := deleteObjectGrants(ctx, tx, model.ObjectTypeBlueprint, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM blueprints WHERE id = $1`, id) // versions cascade
		return err
	})
}

const blueprintVersionColumns = `blueprint_id, version, kind, message, brief, model, checks, notes, ok, created_by, created_at`

func (r *BlueprintRepo) GetVersion(ctx context.Context, id string, version int) (model.BlueprintVersion, error) {
	v, err := scanBlueprintVersion(r.pool.QueryRow(ctx, `SELECT `+blueprintVersionColumns+`
		FROM blueprint_versions WHERE blueprint_id = $1 AND version = $2`, id, version))
	return v, mapErr(err)
}

// scanBlueprintVersion reads a row selected with blueprintVersionColumns (or returned by an INSERT of them).
func scanBlueprintVersion(row pgx.Row) (model.BlueprintVersion, error) {
	var v model.BlueprintVersion
	var brief, plan, checks, notes []byte
	if err := row.Scan(&v.BlueprintID, &v.Version, &v.Kind, &v.Message, &brief, &plan, &checks, &notes, &v.OK, &v.CreatedBy, &v.CreatedAt); err != nil {
		return model.BlueprintVersion{}, err
	}
	v.Brief, v.Model, v.Checks = brief, plan, checks
	if err := json.Unmarshal(notes, &v.Notes); err != nil {
		return model.BlueprintVersion{}, err
	}
	v.Notes = nonNil(v.Notes)
	return v, nil
}

func (r *BlueprintRepo) ListVersions(ctx context.Context, id string) ([]model.BlueprintVersion, error) {
	rows, err := r.pool.Query(ctx, `SELECT blueprint_id, version, kind, message, ok, created_by, created_at
		FROM blueprint_versions WHERE blueprint_id = $1 ORDER BY version DESC`, id)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []model.BlueprintVersion{}
	for rows.Next() {
		var v model.BlueprintVersion
		if err := rows.Scan(&v.BlueprintID, &v.Version, &v.Kind, &v.Message, &v.OK, &v.CreatedBy, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// AppendVersion moves current_version from base to base+1 only while it is
// still base, then inserts the row, in one transaction. It returns the blueprint
// and the version row as that transaction left them.
func (r *BlueprintRepo) AppendVersion(ctx context.Context, v model.BlueprintVersion, base int) (model.Blueprint, model.BlueprintVersion, error) {
	return r.append(ctx, v, base, nil)
}

func (r *BlueprintRepo) AppendVersionWithOriginal(ctx context.Context, v model.BlueprintVersion, base int, o model.BlueprintOriginal) (model.Blueprint, model.BlueprintVersion, error) {
	return r.append(ctx, v, base, &o)
}

func (r *BlueprintRepo) append(ctx context.Context, v model.BlueprintVersion, base int, o *model.BlueprintOriginal) (model.Blueprint, model.BlueprintVersion, error) {
	var b model.Blueprint
	var got model.BlueprintVersion
	err := inTx(ctx, r.pool, func(tx pgx.Tx) error {
		var err error
		b, err = scanBlueprint(tx.QueryRow(ctx, `UPDATE blueprints SET current_version = $2 + 1, updated_at = now() WHERE id = $1 AND current_version = $2
			RETURNING `+blueprintColumns, v.BlueprintID, base))
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Conflict("blueprint changed since version %d; reload and try again", base)
		}
		if err != nil {
			return err
		}
		v.Version = base + 1
		if got, err = insertBlueprintVersion(ctx, tx, v); err != nil {
			return err
		}
		if o == nil {
			return nil
		}
		_, err = tx.Exec(ctx, `INSERT INTO blueprint_originals (blueprint_id, version, name, mime_type, byte_size, sha256, data)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, v.BlueprintID, v.Version, o.Name, o.MimeType, o.ByteSize, o.SHA256, o.Data)
		return err
	})
	if err != nil {
		return model.Blueprint{}, model.BlueprintVersion{}, err
	}
	return b, got, nil
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
