package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func blueprintDraftOutput(m model.BlueprintDraft) output.BlueprintDraft {
	return output.BlueprintDraft{
		Brief: m.Brief,
		Notes: m.Notes,
	}
}
