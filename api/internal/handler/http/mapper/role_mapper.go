package mapper

import (
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type RoleJSON struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	Name        string         `json:"name"`
	Affiliation string         `json:"affiliation"`
	Permissions []string       `json:"permissions"`
	Assignees   []AssigneeJSON `json:"assignees"`
	CreatedAt   time.Time      `json:"created_at"`
}

type AssigneeJSON struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

func RoleView(role model.Role) RoleJSON {
	perms := role.Permissions
	if perms == nil {
		perms = []string{}
	}
	assignees := make([]AssigneeJSON, 0, len(role.Assignees))
	for _, a := range role.Assignees {
		assignees = append(assignees, AssigneeJSON{a.UserID, a.Email, a.Name})
	}
	return RoleJSON{role.ID, role.TenantID, role.Name, role.Affiliation, perms, assignees, role.CreatedAt}
}

func RoleViews(roles []model.Role) []RoleJSON {
	out := make([]RoleJSON, 0, len(roles))
	for _, role := range roles {
		out = append(out, RoleView(role))
	}
	return out
}
