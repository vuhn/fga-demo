package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type EstimateSessionRepo struct{ pool *pgxpool.Pool }

var _ gateway.EstimateSessionGateway = (*EstimateSessionRepo)(nil)

func NewEstimateSessionRepo(pool *pgxpool.Pool) *EstimateSessionRepo {
	return &EstimateSessionRepo{pool: pool}
}

// Replace upserts the blueprint's one session: a first start inserts it, a restart resets the row (state
// cleared, seq 0, epoch + 1) and deletes its messages, in one transaction. The upsert comes first so the row
// lock makes a turn that is saving at that moment either finish before the reset or fail its epoch guard.
func (r *EstimateSessionRepo) Replace(ctx context.Context, s model.EstimateSession) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO estimate_sessions (blueprint_id, blueprint_version, state, created_by)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (blueprint_id) DO UPDATE SET blueprint_version = EXCLUDED.blueprint_version, state = EXCLUDED.state,
				seq = 0, epoch = estimate_sessions.epoch + 1, created_by = EXCLUDED.created_by, created_at = now(), updated_at = now()`,
			s.BlueprintID, s.BlueprintVersion, string(jsonOr(s.State, "{}")), s.CreatedBy); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM estimate_messages WHERE blueprint_id = $1`, s.BlueprintID)
		return err
	})
}

func (r *EstimateSessionRepo) Get(ctx context.Context, blueprintID string) (model.EstimateSession, error) {
	var s model.EstimateSession
	var state string
	err := r.pool.QueryRow(ctx, `SELECT blueprint_id, blueprint_version, epoch, state::text, seq, created_by, created_at, updated_at
		FROM estimate_sessions WHERE blueprint_id = $1`, blueprintID).Scan(&s.BlueprintID, &s.BlueprintVersion, &s.Epoch, &state, &s.Seq, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return model.EstimateSession{}, mapErr(err)
	}
	s.State = json.RawMessage(state)
	return s, nil
}

func (r *EstimateSessionRepo) Messages(ctx context.Context, blueprintID string) ([]model.EstimateMessage, error) {
	rows, err := r.pool.Query(ctx, `SELECT blueprint_id, seq, role, content, created_at FROM estimate_messages WHERE blueprint_id = $1 ORDER BY seq`, blueprintID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []model.EstimateMessage{}
	for rows.Next() {
		var m model.EstimateMessage
		if err := rows.Scan(&m.BlueprintID, &m.Seq, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AppendTurn: the UPDATE only matches while epoch and seq are unchanged, so a concurrent turn that finished
// first, or a restart since this turn began, makes this one a 409 with nothing written.
func (r *EstimateSessionRepo) AppendTurn(ctx context.Context, p model.AppendEstimateTurnParams) error {
	s, est := p.Session, p.Estimate
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE estimate_sessions SET state = $1, seq = seq + 1, updated_at = now() WHERE blueprint_id = $2 AND epoch = $3 AND seq = $4`,
			string(jsonOr(p.State, "{}")), s.BlueprintID, s.Epoch, s.Seq)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return model.Conflict("another turn finished first or the estimate was restarted; reload the session")
		}
		for i, m := range []model.EstimateMessage{p.User, p.Assistant} {
			if _, err := tx.Exec(ctx, `INSERT INTO estimate_messages (blueprint_id, seq, role, content) VALUES ($1, $2, $3, $4)`,
				s.BlueprintID, s.Seq*2+i+1, m.Role, m.Content); err != nil {
				return err
			}
		}
		if est != nil {
			if _, err := tx.Exec(ctx, `INSERT INTO blueprint_estimates (id, blueprint_id, version, total_low, total_base, total_high, result)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`, est.ID, est.BlueprintID, est.Version, est.TotalLow, est.TotalBase, est.TotalHigh, string(est.Result)); err != nil {
				return err
			}
		}
		return nil
	})
}
