package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func blueprintVersionOutput(m model.BlueprintVersion) output.BlueprintVersion {
	return output.BlueprintVersion{
		BlueprintID: m.BlueprintID,
		Version:     m.Version,
		Kind:        m.Kind,
		Message:     m.Message,
		Brief:       m.Brief,
		Model:       m.Model,
		Checks:      m.Checks,
		Notes:       m.Notes,
		OK:          m.OK,
		CreatedBy:   m.CreatedBy,
		CreatedAt:   m.CreatedAt,
	}
}
