package http

import (
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// JSON views shared by several handlers. snake_case, RFC3339 timestamps.
// Each content type keeps its own view next to its handler.

type userJSON struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	IsSystemAdmin bool   `json:"is_system_admin"`
}

func userView(u entity.User) userJSON {
	return userJSON{u.ID, u.Email, u.Name, u.IsSystemAdmin}
}

type productJSON struct {
	Product   string    `json:"product"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

func productsView(ps []entity.Product) []productJSON {
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

func tenantView(t entity.Tenant) tenantJSON {
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

func memberView(m entity.Membership) memberJSON {
	return memberJSON{m.UserID, m.Email, m.Name, m.Affiliation, m.Role}
}

type grantJSON struct {
	ID       string  `json:"id"`
	Relation string  `json:"relation"`
	Subject  string  `json:"subject"`
	UserID   *string `json:"user_id"`
	Name     string  `json:"name,omitempty"`
}

func grantView(g entity.Grant) grantJSON {
	return grantJSON{g.ID, g.Relation, g.SubjectType, g.UserID, g.UserName}
}

// shareBody is the share and revoke request body.
type shareBody struct {
	Relation string `json:"relation"`
	Subject  string `json:"subject"`
	UserID   string `json:"user_id"`
}

func (b shareBody) input() entity.ShareInput {
	return entity.ShareInput{Relation: b.Relation, Subject: b.Subject, UserID: b.UserID}
}

// visibilityBody is the folder and album visibility request body.
type visibilityBody struct {
	IsPrivate *bool `json:"is_private"`
}

type tupleJSON struct {
	User     string `json:"user"`
	Relation string `json:"relation"`
	Object   string `json:"object"`
}
