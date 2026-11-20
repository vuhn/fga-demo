package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func userOutput(m model.User) output.User {
	return output.User{
		ID:            m.ID,
		Email:         m.Email,
		Name:          m.Name,
		IsSystemAdmin: m.IsSystemAdmin,
		CompanyID:     m.CompanyID,
		CompanyName:   m.CompanyName,
		CreatedAt:     m.CreatedAt,
	}
}
