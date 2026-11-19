package http

import (
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type tenantJSON struct {
	ID          string        `json:"id"`
	Slug        string        `json:"slug"`
	Name        string        `json:"name"`
	CreatedAt   time.Time     `json:"created_at"`
	Products    []productJSON `json:"products"`
	Affiliation string        `json:"affiliation,omitempty"`
	Role        string        `json:"role,omitempty"`
	RoleName    string        `json:"role_name,omitempty"`
}

func tenantView(t model.Tenant) tenantJSON {
	return tenantJSON{ID: t.ID, Slug: t.Slug, Name: t.Name, CreatedAt: t.CreatedAt, Products: productsView(t.Products)}
}

func tenantViews(ts []usecase.TenantView) []tenantJSON {
	out := make([]tenantJSON, 0, len(ts))
	for _, t := range ts {
		v := tenantView(t.Tenant)
		v.Affiliation, v.Role, v.RoleName = t.Affiliation, t.Role, t.RoleName
		out = append(out, v)
	}
	return out
}
