package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
	"time"
)

type TenantJSON struct {
	ID          string        `json:"id"`
	Slug        string        `json:"slug"`
	Name        string        `json:"name"`
	CreatedAt   time.Time     `json:"created_at"`
	Products    []ProductJSON `json:"products"`
	Affiliation string        `json:"affiliation,omitempty"`
	Role        string        `json:"role,omitempty"`
	RoleName    string        `json:"role_name,omitempty"`
}

func TenantView(t output.Tenant) TenantJSON {
	return TenantJSON{ID: t.ID, Slug: t.Slug, Name: t.Name, CreatedAt: t.CreatedAt, Products: ProductsView(t.Products)}
}

func TenantViews(ts []output.TenantView) []TenantJSON {
	out := make([]TenantJSON, 0, len(ts))
	for _, t := range ts {
		v := TenantView(t.Tenant)
		v.Affiliation, v.Role, v.RoleName = t.Affiliation, t.Role, t.RoleName
		out = append(out, v)
	}
	return out
}
