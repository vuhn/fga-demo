package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

// CompanyRefJSON is the company shown next to a person everywhere user info is displayed.
type CompanyRefJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserJSON struct {
	ID            string         `json:"id"`
	Email         string         `json:"email"`
	Name          string         `json:"name"`
	IsSystemAdmin bool           `json:"is_system_admin"`
	Company       CompanyRefJSON `json:"company"`
}

func UserView(u output.User) UserJSON {
	return UserJSON{u.ID, u.Email, u.Name, u.IsSystemAdmin, CompanyRefJSON{u.CompanyID, u.CompanyName}}
}
