package usecase

import (
	"context"
	"regexp"
	"strings"

	"github.com/vuhn/fga-demo/internal/domain"
)

var slugRe = regexp.MustCompile(`^[a-z0-9-]{2,40}$`)

func (a *App) requirePlatform(ctx context.Context, actor string) error {
	return a.require(ctx, actor, "can_manage_platform", "platform", domain.PlatformID)
}

// ---- Users ----

func (a *App) ListUsers(ctx context.Context, actor string) ([]domain.User, error) {
	if err := a.requirePlatform(ctx, actor); err != nil {
		return nil, err
	}
	return a.Store.ListUsers(ctx)
}

func (a *App) CreateUser(ctx context.Context, actor, email, name, password string) (domain.User, error) {
	if err := a.requirePlatform(ctx, actor); err != nil {
		return domain.User{}, err
	}
	email = strings.TrimSpace(email)
	if !strings.Contains(email, "@") {
		return domain.User{}, domain.Invalid("email must contain @")
	}
	name, err := cleanName("name", name)
	if err != nil {
		return domain.User{}, err
	}
	if n := len(password); n < 8 || n > 72 {
		return domain.User{}, domain.Invalid("password length must be 8 to 72")
	}
	hash, err := a.Hasher.Hash(password)
	if err != nil {
		return domain.User{}, err
	}
	u := domain.User{ID: a.NewID(), Email: email, Name: name, PasswordHash: hash}
	if err := a.Store.CreateUser(ctx, u); err != nil {
		return domain.User{}, err
	}
	return a.Store.GetUser(ctx, u.ID)
}

func (a *App) UpdateUser(ctx context.Context, actor, id, name string) (domain.User, error) {
	if err := a.requirePlatform(ctx, actor); err != nil {
		return domain.User{}, err
	}
	name, err := cleanName("name", name)
	if err != nil {
		return domain.User{}, err
	}
	return a.Store.UpdateUserName(ctx, id, name)
}

// ---- Tenants ----

func (a *App) ListTenants(ctx context.Context, actor string) ([]domain.Tenant, error) {
	if err := a.requirePlatform(ctx, actor); err != nil {
		return nil, err
	}
	return a.Store.ListTenants(ctx)
}

func (a *App) CreateTenant(ctx context.Context, actor, name, slug string) (domain.Tenant, error) {
	if err := a.requirePlatform(ctx, actor); err != nil {
		return domain.Tenant{}, err
	}
	name, err := cleanName("name", name)
	if err != nil {
		return domain.Tenant{}, err
	}
	if !slugRe.MatchString(slug) {
		return domain.Tenant{}, domain.Invalid("slug must match ^[a-z0-9-]{2,40}$")
	}
	// Products start disabled: no wildcard tuples, so no tuple write at all.
	t := domain.Tenant{ID: a.NewID(), Name: name, Slug: slug}
	if err := a.Store.CreateTenant(ctx, t); err != nil {
		return domain.Tenant{}, err
	}
	return a.Store.GetTenant(ctx, t.ID)
}

func (a *App) GetTenant(ctx context.Context, actor, id string) (domain.Tenant, error) {
	if err := a.platformOr(ctx, actor, "can_read_tenant", id, domain.ErrNotFound); err != nil {
		return domain.Tenant{}, err
	}
	return a.Store.GetTenant(ctx, id)
}

func (a *App) UpdateTenant(ctx context.Context, actor, id, name string) (domain.Tenant, error) {
	if err := a.requirePlatform(ctx, actor); err != nil {
		return domain.Tenant{}, err
	}
	name, err := cleanName("name", name)
	if err != nil {
		return domain.Tenant{}, err
	}
	return a.Store.UpdateTenantName(ctx, id, name)
}

// ---- Products ----

// ListProducts shows product state on the tenant page: the internal admin
// (can_view_products) or the platform may read it.
func (a *App) ListProducts(ctx context.Context, actor, tenantID string) ([]domain.Product, error) {
	if err := a.platformOr(ctx, actor, "can_view_products", tenantID, domain.ErrForbidden); err != nil {
		return nil, err
	}
	if _, err := a.Store.GetTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	return a.Store.ListProducts(ctx, tenantID)
}

// SetProduct writes or deletes the single wildcard tuple. Content tuples are
// never touched. Products are an entitlement the platform grants: only a
// system admin may toggle them; tenant admins can only view them.
func (a *App) SetProduct(ctx context.Context, actor, tenantID, product string, enabled bool) ([]domain.Product, error) {
	if !domain.ValidProduct(product) {
		return nil, domain.Invalid("product must be documents or photos")
	}
	if err := a.requirePlatform(ctx, actor); err != nil {
		return nil, err
	}
	if _, err := a.Store.GetTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	writes, deletes := a.Tuples.SetProduct(tenantID, product, enabled)
	if enabled {
		prev, err := a.Store.SetProduct(ctx, tenantID, product, true)
		if err != nil {
			return nil, err
		}
		err = a.grant(ctx, writes, nil, func(ctx context.Context) error {
			_, err := a.Store.SetProduct(ctx, tenantID, product, prev)
			return err
		})
		if err != nil {
			return nil, err
		}
	} else {
		// Restore only when it was on before; the display flag tells us.
		products, err := a.Store.ListProducts(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		var restore []Tuple
		for _, p := range products {
			if p.Name == product && p.Enabled {
				restore = deletes
			}
		}
		err = a.revoke(ctx, deletes, restore, func(ctx context.Context) error {
			_, err := a.Store.SetProduct(ctx, tenantID, product, false)
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	return a.Store.ListProducts(ctx, tenantID)
}
