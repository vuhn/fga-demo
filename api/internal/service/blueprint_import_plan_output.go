package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func blueprintImportPlanOutput(m model.BlueprintImportPlan) output.BlueprintImportPlan {
	return output.BlueprintImportPlan{
		Index:     m.Index,
		Name:      m.Name,
		RoomCount: m.RoomCount,
		NetArea:   m.NetArea,
		OK:        m.OK,
		Model:     m.Model,
		Checks:    m.Checks,
		SVG:       m.SVG,
	}
}
