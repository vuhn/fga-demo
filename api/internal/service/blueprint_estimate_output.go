package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func blueprintEstimateOutput(m model.BlueprintEstimate) output.BlueprintEstimate {
	return output.BlueprintEstimate{
		ID:          m.ID,
		BlueprintID: m.BlueprintID,
		Version:     m.Version,
		TotalLow:    m.TotalLow,
		TotalBase:   m.TotalBase,
		TotalHigh:   m.TotalHigh,
		Result:      m.Result,
		CreatedAt:   m.CreatedAt,
	}
}
