package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
	"time"
)

type ProjectJSON struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedBy string    `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func ProjectView(p output.Project) ProjectJSON {
	return ProjectJSON{p.ID, p.TenantID, p.Name, p.CreatedBy, p.CreatedAt}
}

func VisibleProjectView(v output.Visible[output.Project]) map[string]any {
	p := v.Value
	return map[string]any{
		"id": p.ID, "tenant_id": p.TenantID, "name": p.Name, "created_by": p.CreatedBy, "created_at": p.CreatedAt,
		"permissions": v.Perms,
	}
}
