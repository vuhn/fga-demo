package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func projectMemberOutput(m model.ProjectMember) output.ProjectMember {
	return output.ProjectMember{
		ProjectID:     m.ProjectID,
		TenantID:      m.TenantID,
		UserID:        m.UserID,
		Affiliation:   m.Affiliation,
		Role:          m.Role,
		RoleName:      m.RoleName,
		ProjectRoleID: m.ProjectRoleID,
		Email:         m.Email,
		Name:          m.Name,
		CompanyID:     m.CompanyID,
		CompanyName:   m.CompanyName,
	}
}
