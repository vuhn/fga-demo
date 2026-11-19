package http

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type memberJSON struct {
	UserID      string         `json:"user_id"`
	Email       string         `json:"email"`
	Name        string         `json:"name"`
	Affiliation string         `json:"affiliation"`
	Role        string         `json:"role"`
	RoleName    string         `json:"role_name,omitempty"`
	Company     companyRefJSON `json:"company"`
}

func memberView(m model.Membership) memberJSON {
	return memberJSON{m.UserID, m.Email, m.Name, m.Affiliation, m.Role, m.RoleName, companyRefJSON{m.CompanyID, m.CompanyName}}
}
