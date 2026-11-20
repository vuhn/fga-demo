package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func projectOutput(m model.Project) output.Project {
	return output.Project{
		ID:        m.ID,
		TenantID:  m.TenantID,
		Name:      m.Name,
		CreatedBy: m.CreatedBy,
		CreatedAt: m.CreatedAt,
	}
}
