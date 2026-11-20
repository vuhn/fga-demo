package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type roleCreator struct {
	tenants   gateway.TenantGateway
	roles     gateway.RoleGateway
	relations gateway.RoleRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewRoleCreator(tenants gateway.TenantGateway, roles gateway.RoleGateway, relations gateway.RoleRelationGateway, authz helper.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.RoleCreator {
	return &roleCreator{tenants: tenants, roles: roles, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create adds an empty custom role for one tenant. Platform only.
func (s *roleCreator) Create(ctx context.Context, in input.RoleCreator) (output.RoleCreator, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.RoleCreator{}, err
	}
	name, err := model.CleanRoleName(in.Name)
	if err != nil {
		return output.RoleCreator{}, err
	}
	if in.Affiliation != model.AffiliationInternal && in.Affiliation != model.AffiliationExternal {
		return output.RoleCreator{}, model.Invalid("unknown affiliation %q", in.Affiliation)
	}
	if _, err := s.tenants.Get(ctx, in.TenantID); err != nil {
		return output.RoleCreator{}, err
	}
	role := model.Role{ID: s.ids.NewID(), TenantID: in.TenantID, Name: name, Affiliation: in.Affiliation, Permissions: []string{}, Assignees: []model.RoleAssignee{}}
	tuples, err := s.relations.Created(in.TenantID, role.ID, in.Affiliation)
	if err != nil {
		return output.RoleCreator{}, err
	}
	if err := s.roles.Insert(ctx, role); err != nil {
		return output.RoleCreator{}, err
	}
	err = s.guard.Grant(ctx, tuples, nil, func(ctx context.Context) error {
		return s.roles.Delete(ctx, in.TenantID, role.ID)
	})
	if err != nil {
		return output.RoleCreator{}, err
	}
	got, err := s.roles.Get(ctx, in.TenantID, role.ID)
	return output.RoleCreator{Role: roleOutput(got)}, err
}
