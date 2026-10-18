package http

import (
	"time"

	"github.com/vuhn/fga-demo/internal/domain"
	"github.com/vuhn/fga-demo/internal/usecase"
)

// JSON views. snake_case, RFC3339 timestamps.

type userJSON struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	IsSystemAdmin bool   `json:"is_system_admin"`
}

func userView(u domain.User) userJSON {
	return userJSON{u.ID, u.Email, u.Name, u.IsSystemAdmin}
}

type productJSON struct {
	Product   string    `json:"product"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

func productsView(ps []domain.Product) []productJSON {
	out := make([]productJSON, 0, len(ps))
	for _, p := range ps {
		out = append(out, productJSON{p.Name, p.Enabled, p.UpdatedAt})
	}
	return out
}

type tenantJSON struct {
	ID          string        `json:"id"`
	Slug        string        `json:"slug"`
	Name        string        `json:"name"`
	CreatedAt   time.Time     `json:"created_at"`
	Products    []productJSON `json:"products"`
	Affiliation string        `json:"affiliation,omitempty"`
	Role        string        `json:"role,omitempty"`
}

func tenantView(t domain.Tenant) tenantJSON {
	return tenantJSON{ID: t.ID, Slug: t.Slug, Name: t.Name, CreatedAt: t.CreatedAt, Products: productsView(t.Products)}
}

func tenantViews(ts []usecase.TenantView) []tenantJSON {
	out := make([]tenantJSON, 0, len(ts))
	for _, t := range ts {
		v := tenantView(t.Tenant)
		v.Affiliation, v.Role = t.Affiliation, t.Role
		out = append(out, v)
	}
	return out
}

type memberJSON struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Affiliation string `json:"affiliation"`
	Role        string `json:"role"`
}

func memberView(m domain.Membership) memberJSON {
	return memberJSON{m.UserID, m.Email, m.Name, m.Affiliation, m.Role}
}

func containerView(c domain.Container) map[string]any {
	return map[string]any{
		"id": c.ID, "tenant_id": c.TenantID, "parent_id": c.ParentID, "name": c.Name, "is_private": c.IsPrivate,
		"created_by": c.CreatedBy, "created_at": c.CreatedAt,
	}
}

func visibleContainerView(v usecase.Visible[domain.Container]) map[string]any {
	m := containerView(v.Value)
	m["permissions"] = v.Perms
	return m
}

func itemView(kind domain.Kind, it domain.Item) map[string]any {
	return map[string]any{
		"id": it.ID, "tenant_id": it.TenantID, kind.ContainerRef + "_id": it.ContainerID, "name": it.Name,
		"mime_type": it.MimeType, "byte_size": it.ByteSize, "created_by": it.CreatedBy, "created_at": it.CreatedAt,
	}
}

type grantJSON struct {
	ID       string  `json:"id"`
	Relation string  `json:"relation"`
	Subject  string  `json:"subject"`
	UserID   *string `json:"user_id"`
	Name     string  `json:"name,omitempty"`
}

func grantView(g domain.Grant) grantJSON {
	return grantJSON{g.ID, g.Relation, g.SubjectType, g.UserID, g.UserName}
}

type tupleJSON struct {
	User     string `json:"user"`
	Relation string `json:"relation"`
	Object   string `json:"object"`
}

func mapSlice[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}
