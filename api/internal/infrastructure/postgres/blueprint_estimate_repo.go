package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type BlueprintEstimateRepo struct{ pool *pgxpool.Pool }

var _ gateway.BlueprintEstimateGateway = (*BlueprintEstimateRepo)(nil)

func NewBlueprintEstimateRepo(pool *pgxpool.Pool) *BlueprintEstimateRepo {
	return &BlueprintEstimateRepo{pool: pool}
}

func (r *BlueprintEstimateRepo) Get(ctx context.Context, blueprintID, id string) (model.BlueprintEstimate, error) {
	var e model.BlueprintEstimate
	var result string
	err := r.pool.QueryRow(ctx, `SELECT id, blueprint_id, version, total_low, total_base, total_high, result::text, created_at
		FROM blueprint_estimates WHERE blueprint_id = $1 AND id = $2`, blueprintID, id).
		Scan(&e.ID, &e.BlueprintID, &e.Version, &e.TotalLow, &e.TotalBase, &e.TotalHigh, &result, &e.CreatedAt)
	if err != nil {
		return model.BlueprintEstimate{}, mapErr(err)
	}
	e.Result = json.RawMessage(result)
	return e, nil
}

func (r *BlueprintEstimateRepo) ListByBlueprint(ctx context.Context, blueprintID string) ([]model.BlueprintEstimate, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, blueprint_id, version, total_low, total_base, total_high, created_at
		FROM blueprint_estimates WHERE blueprint_id = $1 ORDER BY created_at DESC`, blueprintID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []model.BlueprintEstimate{}
	for rows.Next() {
		var e model.BlueprintEstimate
		if err := rows.Scan(&e.ID, &e.BlueprintID, &e.Version, &e.TotalLow, &e.TotalBase, &e.TotalHigh, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
