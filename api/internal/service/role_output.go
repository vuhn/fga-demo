package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func roleOutput(m model.Role) output.Role {
	return output.Role{
		ID:          m.ID,
		TenantID:    m.TenantID,
		Name:        m.Name,
		Affiliation: m.Affiliation,
		Permissions: m.Permissions,
		Assignees:   mapOutput(m.Assignees, roleAssigneeOutput),
		CreatedAt:   m.CreatedAt,
	}
}
