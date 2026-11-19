package http

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// companyRefJSON is the company shown next to a person everywhere user info is displayed.
type companyRefJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type userJSON struct {
	ID            string         `json:"id"`
	Email         string         `json:"email"`
	Name          string         `json:"name"`
	IsSystemAdmin bool           `json:"is_system_admin"`
	Company       companyRefJSON `json:"company"`
}

func userView(u model.User) userJSON {
	return userJSON{u.ID, u.Email, u.Name, u.IsSystemAdmin, companyRefJSON{u.CompanyID, u.CompanyName}}
}
