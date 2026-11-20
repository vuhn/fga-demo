package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type blueprintGrantLister struct {
	blueprints gateway.BlueprintGateway
	grants     gateway.BlueprintGrantGateway
	authz      helper.Authorizer
}

func NewBlueprintGrantLister(blueprints gateway.BlueprintGateway, grants gateway.BlueprintGrantGateway, authz helper.Authorizer) usecase.BlueprintGrantLister {
	return &blueprintGrantLister{blueprints: blueprints, grants: grants, authz: authz}
}

func (s *blueprintGrantLister) ListGrants(ctx context.Context, in input.BlueprintGrantLister) (output.BlueprintGrantLister, error) {
	if err := requireBlueprintShare(ctx, s.blueprints, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return output.BlueprintGrantLister{}, err
	}
	got, err := s.grants.List(ctx, in.ID)
	return output.BlueprintGrantLister{Items: mapOutput(got, grantOutput)}, err
}
