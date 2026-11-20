package service

import (
	"context"
	"regexp"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

var slugRe = regexp.MustCompile(`^[a-z0-9-]{2,40}$`)

type tenantCreator struct {
	tenants gateway.TenantGateway
	authz   helper.Authorizer
	ids     gateway.IDGenerator
}

func NewTenantCreator(tenants gateway.TenantGateway, authz helper.Authorizer, ids gateway.IDGenerator) usecase.TenantCreator {
	return &tenantCreator{tenants: tenants, authz: authz, ids: ids}
}

// Create is platform only. Products start disabled: no wildcard tuples, so
// no tuple write at all.
func (s *tenantCreator) Create(ctx context.Context, in input.TenantCreator) (output.TenantCreator, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.TenantCreator{}, err
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.TenantCreator{}, err
	}
	if !slugRe.MatchString(in.Slug) {
		return output.TenantCreator{}, model.Invalid("slug must match ^[a-z0-9-]{2,40}$")
	}
	t := model.Tenant{ID: s.ids.NewID(), Name: name, Slug: in.Slug}
	if err := s.tenants.Create(ctx, t); err != nil {
		return output.TenantCreator{}, err
	}
	got, err := s.tenants.Get(ctx, t.ID)
	return output.TenantCreator{Tenant: tenantOutput(got)}, err
}
