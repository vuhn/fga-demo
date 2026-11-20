package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
	"time"
)

type ProjectRoleJSON struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	ProjectID   string         `json:"project_id"`
	Name        string         `json:"name"`
	Affiliation string         `json:"affiliation"`
	Permissions []string       `json:"permissions"`
	Assignees   []AssigneeJSON `json:"assignees"`
	CreatedAt   time.Time      `json:"created_at"`
}

func ProjectRoleView(role output.ProjectRole) ProjectRoleJSON {
	perms := role.Permissions
	if perms == nil {
		perms = []string{}
	}
	assignees := make([]AssigneeJSON, 0, len(role.Assignees))
	for _, a := range role.Assignees {
		assignees = append(assignees, AssigneeJSON{a.UserID, a.Email, a.Name})
	}
	return ProjectRoleJSON{role.ID, role.TenantID, role.ProjectID, role.Name, role.Affiliation, perms, assignees, role.CreatedAt}
}
