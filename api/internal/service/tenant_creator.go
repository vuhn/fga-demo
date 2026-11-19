package service

import (
	"context"
	"regexp"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

var slugRe = regexp.MustCompile(`^[a-z0-9-]{2,40}$`)

type tenantCreator struct {
	tenants gateway.TenantGateway
	authz   usecase.Authorizer
	ids     gateway.IDGenerator
}

func NewTenantCreator(tenants gateway.TenantGateway, authz usecase.Authorizer, ids gateway.IDGenerator) usecase.TenantCreator {
	return &tenantCreator{tenants: tenants, authz: authz, ids: ids}
}

// Create is platform only. Products start disabled: no wildcard tuples, so
// no tuple write at all.
func (s *tenantCreator) Create(ctx context.Context, actor, name, slug string) (model.Tenant, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return model.Tenant{}, err
	}
	name, err := model.CleanName("name", name)
	if err != nil {
		return model.Tenant{}, err
	}
	if !slugRe.MatchString(slug) {
		return model.Tenant{}, model.Invalid("slug must match ^[a-z0-9-]{2,40}$")
	}
	t := model.Tenant{ID: s.ids.NewID(), Name: name, Slug: slug}
	if err := s.tenants.Create(ctx, t); err != nil {
		return model.Tenant{}, err
	}
	return s.tenants.Get(ctx, t.ID)
}
