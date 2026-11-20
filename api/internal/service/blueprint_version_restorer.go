package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type blueprintVersionRestorer struct {
	blueprints gateway.BlueprintGateway
	authz      helper.Authorizer
}

func NewBlueprintVersionRestorer(blueprints gateway.BlueprintGateway, authz helper.Authorizer) usecase.BlueprintVersionRestorer {
	return &blueprintVersionRestorer{blueprints: blueprints, authz: authz}
}

// Restore requires can_edit and a current base_version. It copies an older
// version into a new one; history is never rewritten and the engine is not
// called.
func (s *blueprintVersionRestorer) Restore(ctx context.Context, in input.BlueprintVersionRestorer) (output.BlueprintVersionRestorer, error) {
	b, _, err := loadBlueprintForEdit(ctx, s.blueprints, s.authz, in.Actor, in.TenantID, in.ID, in.BaseVersion)
	if err != nil {
		return output.BlueprintVersionRestorer{}, err
	}
	if in.Version < 1 || in.Version >= b.CurrentVersion {
		return output.BlueprintVersionRestorer{}, model.Invalid("version must be an earlier version (1 to %d)", b.CurrentVersion-1)
	}
	old, err := s.blueprints.GetVersion(ctx, in.ID, in.Version)
	if err != nil {
		return output.BlueprintVersionRestorer{}, err
	}
	v := old
	v.Kind, v.Message, v.CreatedBy = model.BlueprintVersionRestore, fmt.Sprintf("restored version %d", in.Version), in.Actor
	got, err := appendBlueprintVersion(ctx, s.blueprints, b, v, in.BaseVersion)
	return output.BlueprintVersionRestorer{BlueprintDetail: blueprintDetailOutput(got)}, err
}
