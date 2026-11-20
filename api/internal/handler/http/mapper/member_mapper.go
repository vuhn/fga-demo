package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type MemberJSON struct {
	UserID      string         `json:"user_id"`
	Email       string         `json:"email"`
	Name        string         `json:"name"`
	Affiliation string         `json:"affiliation"`
	Role        string         `json:"role"`
	RoleName    string         `json:"role_name,omitempty"`
	Company     CompanyRefJSON `json:"company"`
}

func MemberView(m output.Membership) MemberJSON {
	return MemberJSON{m.UserID, m.Email, m.Name, m.Affiliation, m.Role, m.RoleName, CompanyRefJSON{m.CompanyID, m.CompanyName}}
}
