package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func projectRoleOutput(m model.ProjectRole) output.ProjectRole {
	return output.ProjectRole{
		ID:          m.ID,
		TenantID:    m.TenantID,
		ProjectID:   m.ProjectID,
		Name:        m.Name,
		Affiliation: m.Affiliation,
		Permissions: m.Permissions,
		Assignees:   mapOutput(m.Assignees, roleAssigneeOutput),
		CreatedAt:   m.CreatedAt,
	}
}
