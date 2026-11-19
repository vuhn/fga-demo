package http

import (
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type projectJSON struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedBy string    `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func projectView(p model.Project) projectJSON {
	return projectJSON{p.ID, p.TenantID, p.Name, p.CreatedBy, p.CreatedAt}
}

func visibleProjectView(v model.Visible[model.Project]) map[string]any {
	p := v.Value
	return map[string]any{
		"id": p.ID, "tenant_id": p.TenantID, "name": p.Name, "created_by": p.CreatedBy, "created_at": p.CreatedAt,
		"permissions": v.Perms,
	}
}
