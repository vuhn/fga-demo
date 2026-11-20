package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func roleAssigneeOutput(m model.RoleAssignee) output.RoleAssignee {
	return output.RoleAssignee{
		UserID: m.UserID,
		Email:  m.Email,
		Name:   m.Name,
	}
}
