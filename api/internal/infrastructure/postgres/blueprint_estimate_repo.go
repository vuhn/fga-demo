package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

type BlueprintEstimateRepo struct{ pool *pgxpool.Pool }

var _ gateway.BlueprintEstimateGateway = (*BlueprintEstimateRepo)(nil)

func NewBlueprintEstimateRepo(pool *pgxpool.Pool) *BlueprintEstimateRepo {
	return &BlueprintEstimateRepo{pool: pool}
}

func (r *BlueprintEstimateRepo) Get(ctx context.Context, blueprintID, id string) (entity.BlueprintEstimate, error) {
	var e entity.BlueprintEstimate
	var result string
	err := r.pool.QueryRow(ctx, `SELECT id, session_id, blueprint_id, version, total_low, total_base, total_high, result::text, created_at
		FROM blueprint_estimates WHERE blueprint_id = $1 AND id = $2`, blueprintID, id).
		Scan(&e.ID, &e.SessionID, &e.BlueprintID, &e.Version, &e.TotalLow, &e.TotalBase, &e.TotalHigh, &result, &e.CreatedAt)
	if err != nil {
		return entity.BlueprintEstimate{}, mapErr(err)
	}
	e.Result = json.RawMessage(result)
	return e, nil
}

func (r *BlueprintEstimateRepo) ListByBlueprint(ctx context.Context, blueprintID string) ([]entity.BlueprintEstimate, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, session_id, blueprint_id, version, total_low, total_base, total_high, created_at
		FROM blueprint_estimates WHERE blueprint_id = $1 ORDER BY created_at DESC`, blueprintID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []entity.BlueprintEstimate{}
	for rows.Next() {
		var e entity.BlueprintEstimate
		if err := rows.Scan(&e.ID, &e.SessionID, &e.BlueprintID, &e.Version, &e.TotalLow, &e.TotalBase, &e.TotalHigh, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
