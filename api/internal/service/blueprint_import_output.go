package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func blueprintImportOutput(m model.BlueprintImport) output.BlueprintImport {
	return output.BlueprintImport{
		Plans:      mapOutput(m.Plans, blueprintImportPlanOutput),
		Report:     m.Report,
		Conversion: m.Conversion,
	}
}
