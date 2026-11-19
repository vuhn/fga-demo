package mapper

import (
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type ProjectJSON struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedBy string    `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func ProjectView(p model.Project) ProjectJSON {
	return ProjectJSON{p.ID, p.TenantID, p.Name, p.CreatedBy, p.CreatedAt}
}

func VisibleProjectView(v model.Visible[model.Project]) map[string]any {
	p := v.Value
	return map[string]any{
		"id": p.ID, "tenant_id": p.TenantID, "name": p.Name, "created_by": p.CreatedBy, "created_at": p.CreatedAt,
		"permissions": v.Perms,
	}
}
