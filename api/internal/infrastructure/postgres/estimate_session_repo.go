package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type EstimateSessionRepo struct{ pool *pgxpool.Pool }

var _ gateway.EstimateSessionGateway = (*EstimateSessionRepo)(nil)

func NewEstimateSessionRepo(pool *pgxpool.Pool) *EstimateSessionRepo {
	return &EstimateSessionRepo{pool: pool}
}

func (r *EstimateSessionRepo) Create(ctx context.Context, s entity.EstimateSession) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO estimate_sessions (id, tenant_id, blueprint_id, blueprint_version, status, state, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, s.ID, s.TenantID, s.BlueprintID, s.BlueprintVersion, s.Status, string(jsonOr(s.State, "{}")), s.CreatedBy)
	return mapErr(err)
}

func (r *EstimateSessionRepo) Get(ctx context.Context, id string) (entity.EstimateSession, error) {
	var s entity.EstimateSession
	var state string
	err := r.pool.QueryRow(ctx, `SELECT id, tenant_id, blueprint_id, blueprint_version, status, state::text, seq, created_by, created_at, updated_at
		FROM estimate_sessions WHERE id = $1`, id).Scan(&s.ID, &s.TenantID, &s.BlueprintID, &s.BlueprintVersion, &s.Status, &state, &s.Seq, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return entity.EstimateSession{}, mapErr(err)
	}
	s.State = json.RawMessage(state)
	return s, nil
}

func (r *EstimateSessionRepo) ListByBlueprint(ctx context.Context, blueprintID string) ([]entity.EstimateSession, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, blueprint_id, blueprint_version, status, seq, created_by, created_at, updated_at
		FROM estimate_sessions WHERE blueprint_id = $1 ORDER BY created_at DESC`, blueprintID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.EstimateSession{}
	for rows.Next() {
		var s entity.EstimateSession
		if err := rows.Scan(&s.ID, &s.TenantID, &s.BlueprintID, &s.BlueprintVersion, &s.Status, &s.Seq, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *EstimateSessionRepo) Messages(ctx context.Context, sessionID string) ([]entity.EstimateMessage, error) {
	rows, err := r.pool.Query(ctx, `SELECT session_id, seq, role, content, created_at FROM estimate_messages WHERE session_id = $1 ORDER BY seq`, sessionID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.EstimateMessage{}
	for rows.Next() {
		var m entity.EstimateMessage
		if err := rows.Scan(&m.SessionID, &m.Seq, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AppendTurn: the UPDATE only matches while seq is unchanged, so a concurrent turn that finished first makes
// this one a 409 with nothing written.
func (r *EstimateSessionRepo) AppendTurn(ctx context.Context, s entity.EstimateSession, seq int, user, assistant entity.EstimateMessage, est *entity.BlueprintEstimate) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE estimate_sessions SET state = $1, status = $2, seq = seq + 1, updated_at = now() WHERE id = $3 AND seq = $4`,
			string(jsonOr(s.State, "{}")), s.Status, s.ID, seq)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return entity.Conflict("another turn finished first; reload the session")
		}
		for i, m := range []entity.EstimateMessage{user, assistant} {
			if _, err := tx.Exec(ctx, `INSERT INTO estimate_messages (session_id, seq, role, content) VALUES ($1, $2, $3, $4)`,
				s.ID, seq*2+i+1, m.Role, m.Content); err != nil {
				return err
			}
		}
		if est != nil {
			if _, err := tx.Exec(ctx, `INSERT INTO blueprint_estimates (id, session_id, blueprint_id, version, total_low, total_base, total_high, result)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, est.ID, est.SessionID, est.BlueprintID, est.Version, est.TotalLow, est.TotalBase, est.TotalHigh, string(est.Result)); err != nil {
				return err
			}
		}
		return nil
	})
}
