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

type blueprintRenamer struct {
	blueprints gateway.BlueprintGateway
	authz      helper.Authorizer
}

func NewBlueprintRenamer(blueprints gateway.BlueprintGateway, authz helper.Authorizer) usecase.BlueprintRenamer {
	return &blueprintRenamer{blueprints: blueprints, authz: authz}
}

// Rename requires can_edit.
func (s *blueprintRenamer) Rename(ctx context.Context, in input.BlueprintRenamer) (output.BlueprintRenamer, error) {
	if _, err := loadBlueprint(ctx, s.blueprints, in.TenantID, in.ID); err != nil {
		return output.BlueprintRenamer{}, err
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.BlueprintRenamer{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_edit", model.ObjectTypeBlueprint, in.ID); err != nil {
		return output.BlueprintRenamer{}, err
	}
	got, err := s.blueprints.Rename(ctx, in.ID, name)
	return output.BlueprintRenamer{Blueprint: blueprintOutput(got)}, err
}
