package http

import (
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type projectRoleJSON struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	ProjectID   string         `json:"project_id"`
	Name        string         `json:"name"`
	Affiliation string         `json:"affiliation"`
	Permissions []string       `json:"permissions"`
	Assignees   []assigneeJSON `json:"assignees"`
	CreatedAt   time.Time      `json:"created_at"`
}

func projectRoleView(role model.ProjectRole) projectRoleJSON {
	perms := role.Permissions
	if perms == nil {
		perms = []string{}
	}
	assignees := make([]assigneeJSON, 0, len(role.Assignees))
	for _, a := range role.Assignees {
		assignees = append(assignees, assigneeJSON{a.UserID, a.Email, a.Name})
	}
	return projectRoleJSON{role.ID, role.TenantID, role.ProjectID, role.Name, role.Affiliation, perms, assignees, role.CreatedAt}
}
