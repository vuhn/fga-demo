package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/vuhn/fga-demo/internal/domain"
)

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	User      domain.User
}

// TenantView is a tenant plus the actor's display labels for it.
type TenantView struct {
	domain.Tenant
	Affiliation string
	Role        string
}

func (a *App) Login(ctx context.Context, email, password string) (LoginResult, error) {
	u, err := a.Store.GetUserByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		return LoginResult{}, domain.ErrUnauthenticated
	}
	if err != nil {
		return LoginResult{}, err
	}
	if !a.Hasher.Compare(u.PasswordHash, password) {
		return LoginResult{}, domain.ErrUnauthenticated
	}
	if u, err = a.withSystemAdmin(ctx, u); err != nil {
		return LoginResult{}, err
	}
	tok, exp, err := a.Tokens.Issue(u.ID)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: tok, ExpiresAt: exp, User: u}, nil
}

// withSystemAdmin sets IsSystemAdmin from the OpenFGA Check, not the table.
func (a *App) withSystemAdmin(ctx context.Context, u domain.User) (domain.User, error) {
	ok, err := a.check(ctx, u.ID, "can_manage_platform", "platform", domain.PlatformID)
	u.IsSystemAdmin = ok
	return u, err
}

func (a *App) Me(ctx context.Context, actor string) (domain.User, []TenantView, error) {
	u, err := a.Store.GetUser(ctx, actor)
	if err != nil {
		return domain.User{}, nil, err
	}
	if u, err = a.withSystemAdmin(ctx, u); err != nil {
		return domain.User{}, nil, err
	}
	tenants, err := a.MyTenants(ctx, actor)
	return u, tenants, err
}

// MyTenants is ListObjects can_read_tenant, then a Postgres load of those ids
// with the membership labels.
func (a *App) MyTenants(ctx context.Context, actor string) ([]TenantView, error) {
	ids, err := a.Authz.ListObjectIDs(ctx, actor, "can_read_tenant", "tenant")
	if err != nil {
		return nil, err
	}
	out := []TenantView{}
	if len(ids) == 0 {
		return out, nil
	}
	ts, err := a.Store.TenantsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	ms, err := a.Store.MembershipsForUser(ctx, actor)
	if err != nil {
		return nil, err
	}
	labels := map[string]domain.Membership{}
	for _, m := range ms {
		labels[m.TenantID] = m
	}
	for _, t := range ts {
		m := labels[t.ID]
		out = append(out, TenantView{Tenant: t, Affiliation: m.Affiliation, Role: m.Role})
	}
	return out, nil
}
