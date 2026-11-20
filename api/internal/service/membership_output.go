package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func membershipOutput(m model.Membership) output.Membership {
	return output.Membership{
		TenantID:     m.TenantID,
		UserID:       m.UserID,
		Affiliation:  m.Affiliation,
		CompanyID:    m.CompanyID,
		CompanyName:  m.CompanyName,
		Role:         m.Role,
		RoleName:     m.RoleName,
		CustomRoleID: m.CustomRoleID,
		Email:        m.Email,
		Name:         m.Name,
	}
}
