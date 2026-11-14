package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintGrantLister struct {
	blueprints gateway.BlueprintGateway
	grants     gateway.BlueprintGrantGateway
	authz      usecase.Authorizer
}

func NewBlueprintGrantLister(blueprints gateway.BlueprintGateway, grants gateway.BlueprintGrantGateway, authz usecase.Authorizer) usecase.BlueprintGrantLister {
	return &blueprintGrantLister{blueprints: blueprints, grants: grants, authz: authz}
}

func (s *blueprintGrantLister) ListGrants(ctx context.Context, actor, tenantID, id string) ([]entity.Grant, error) {
	if err := requireBlueprintShare(ctx, s.blueprints, s.authz, actor, tenantID, id); err != nil {
		return nil, err
	}
	return s.grants.List(ctx, id)
}
