package http

import (
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type roleJSON struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	Name        string         `json:"name"`
	Affiliation string         `json:"affiliation"`
	Permissions []string       `json:"permissions"`
	Assignees   []assigneeJSON `json:"assignees"`
	CreatedAt   time.Time      `json:"created_at"`
}

type assigneeJSON struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

func roleView(role model.Role) roleJSON {
	perms := role.Permissions
	if perms == nil {
		perms = []string{}
	}
	assignees := make([]assigneeJSON, 0, len(role.Assignees))
	for _, a := range role.Assignees {
		assignees = append(assignees, assigneeJSON{a.UserID, a.Email, a.Name})
	}
	return roleJSON{role.ID, role.TenantID, role.Name, role.Affiliation, perms, assignees, role.CreatedAt}
}

func roleViews(roles []model.Role) []roleJSON {
	out := make([]roleJSON, 0, len(roles))
	for _, role := range roles {
		out = append(out, roleView(role))
	}
	return out
}
