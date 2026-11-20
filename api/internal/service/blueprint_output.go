package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func blueprintOutput(m model.Blueprint) output.Blueprint {
	var source *output.BlueprintSource
	if m.Source != nil {
		copied := blueprintSourceOutput(*m.Source)
		source = &copied
	}
	return output.Blueprint{
		ID:             m.ID,
		TenantID:       m.TenantID,
		ProjectID:      m.ProjectID,
		Name:           m.Name,
		CurrentVersion: m.CurrentVersion,
		SourceKind:     m.SourceKind,
		Source:         source,
		CreatedBy:      m.CreatedBy,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func blueprintDetailOutput(d blueprintDetail) output.BlueprintDetail {
	return output.BlueprintDetail{Blueprint: blueprintOutput(d.Blueprint), Perms: d.Perms, Version: blueprintVersionOutput(d.Version)}
}
