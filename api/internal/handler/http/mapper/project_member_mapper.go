package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type ProjectMemberJSON struct {
	UserID      string         `json:"user_id"`
	Email       string         `json:"email"`
	Name        string         `json:"name"`
	Affiliation string         `json:"affiliation"`
	Role        string         `json:"role"`
	RoleName    string         `json:"role_name,omitempty"`
	Company     CompanyRefJSON `json:"company"`
}

func ProjectMemberView(m model.ProjectMember) ProjectMemberJSON {
	return ProjectMemberJSON{m.UserID, m.Email, m.Name, m.Affiliation, m.Role, m.RoleName, CompanyRefJSON{m.CompanyID, m.CompanyName}}
}
