package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type roleCreator struct {
	tenants   gateway.TenantGateway
	roles     gateway.RoleGateway
	relations gateway.RoleRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewRoleCreator(tenants gateway.TenantGateway, roles gateway.RoleGateway, relations gateway.RoleRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.RoleCreator {
	return &roleCreator{tenants: tenants, roles: roles, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create adds an empty custom role for one tenant. Platform only.
func (s *roleCreator) Create(ctx context.Context, actor, tenantID, name, affiliation string) (model.Role, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return model.Role{}, err
	}
	name, err := model.CleanRoleName(name)
	if err != nil {
		return model.Role{}, err
	}
	if affiliation != model.AffiliationInternal && affiliation != model.AffiliationExternal {
		return model.Role{}, model.Invalid("unknown affiliation %q", affiliation)
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return model.Role{}, err
	}
	role := model.Role{ID: s.ids.NewID(), TenantID: tenantID, Name: name, Affiliation: affiliation, Permissions: []string{}, Assignees: []model.RoleAssignee{}}
	tuples, err := s.relations.Created(tenantID, role.ID, affiliation)
	if err != nil {
		return model.Role{}, err
	}
	if err := s.roles.Insert(ctx, role); err != nil {
		return model.Role{}, err
	}
	err = s.guard.Grant(ctx, tuples, nil, func(ctx context.Context) error {
		return s.roles.Delete(ctx, tenantID, role.ID)
	})
	if err != nil {
		return model.Role{}, err
	}
	return s.roles.Get(ctx, tenantID, role.ID)
}
