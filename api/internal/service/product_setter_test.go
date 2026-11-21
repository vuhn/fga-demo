package service

import (
	"context"
	"errors"
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type psTenants struct{ gateway.TenantGateway }

func (psTenants) Get(context.Context, string) (model.Tenant, error) { return model.Tenant{}, nil }

type psProducts struct {
	gateway.ProductGateway
	on map[string]bool
}

func (p *psProducts) Set(_ context.Context, _, product string, enabled bool) (bool, error) {
	prev := p.on[product]
	p.on[product] = enabled
	return prev, nil
}
func (p *psProducts) List(context.Context, string) ([]model.Product, error) {
	out := []model.Product{}
	for n, e := range p.on {
		out = append(out, model.Product{Name: n, Enabled: e})
	}
	return out, nil
}

type psRelations struct{}

func (psRelations) Enabled(tenantID, product string) model.Tuple {
	return model.Tuple{User: "user:*", Relation: product + "_enabled", Object: "tenant:" + tenantID}
}

type psAuthz struct {
	helper.Authorizer
	deny bool
}

func (a psAuthz) RequirePlatform(context.Context, string) error {
	if a.deny {
		return model.ErrForbidden
	}
	return nil
}

type psGuard struct {
	helper.WriteGuard
	granted, revoked []model.Tuple
}

func (g *psGuard) Grant(ctx context.Context, w, _ []model.Tuple, _ func(context.Context) error) error {
	g.granted = append(g.granted, w...)
	return nil
}
func (g *psGuard) Revoke(ctx context.Context, d, _ []model.Tuple, persist func(context.Context) error) error {
	g.revoked = append(g.revoked, d...)
	return persist(ctx)
}

func newSetter(deny bool) (*productSetter, *psProducts, *psGuard) {
	p, g := &psProducts{on: map[string]bool{}}, &psGuard{}
	return NewProductSetter(psTenants{}, p, psRelations{}, psAuthz{deny: deny}, g).(*productSetter), p, g
}

func TestProductSetterTasks(t *testing.T) {
	ctx := context.Background()
	s, p, g := newSetter(false)
	if _, err := s.Set(ctx, input.ProductSetter{Actor: "a", TenantID: "T", Product: "tasks", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if !p.on["tasks"] || len(g.granted) != 1 || g.granted[0].Relation != "tasks_enabled" || g.granted[0].User != "user:*" {
		t.Fatalf("enable: on=%v granted=%+v", p.on, g.granted)
	}
	if _, err := s.Set(ctx, input.ProductSetter{Actor: "a", TenantID: "T", Product: "tasks", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if p.on["tasks"] || len(g.revoked) != 1 || g.revoked[0].Relation != "tasks_enabled" {
		t.Fatalf("disable: on=%v revoked=%+v", p.on, g.revoked)
	}
}

func TestProductSetterRefusals(t *testing.T) {
	s, p, g := newSetter(true)
	if _, err := s.Set(context.Background(), input.ProductSetter{Actor: "a", TenantID: "T", Product: "tasks", Enabled: true}); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("non-platform: %v", err)
	}
	if len(p.on) != 0 || len(g.granted) != 0 {
		t.Fatal("nothing may be written when the Check fails")
	}
	s2, _, _ := newSetter(false)
	if _, err := s2.Set(context.Background(), input.ProductSetter{Actor: "a", TenantID: "T", Product: "billing"}); !errors.Is(err, model.ErrValidation) {
		t.Fatalf("unknown product: %v", err)
	}
}
