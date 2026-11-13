package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintVersionRestorer struct {
	blueprints gateway.BlueprintGateway
	authz      usecase.Authorizer
}

func NewBlueprintVersionRestorer(blueprints gateway.BlueprintGateway, authz usecase.Authorizer) usecase.BlueprintVersionRestorer {
	return &blueprintVersionRestorer{blueprints: blueprints, authz: authz}
}

// Restore requires can_edit and a current base_version. It copies an older
// version into a new one; history is never rewritten and the engine is not
// called.
func (s *blueprintVersionRestorer) Restore(ctx context.Context, actor, tenantID, id string, version, baseVersion int) (usecase.BlueprintDetail, error) {
	b, _, err := loadBlueprintForEdit(ctx, s.blueprints, s.authz, actor, tenantID, id, baseVersion)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if version < 1 || version >= b.CurrentVersion {
		return usecase.BlueprintDetail{}, entity.Invalid("version must be an earlier version (1 to %d)", b.CurrentVersion-1)
	}
	old, err := s.blueprints.GetVersion(ctx, id, version)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	v := old
	v.Kind, v.Message, v.CreatedBy = entity.BlueprintVersionRestore, fmt.Sprintf("restored version %d", version), actor
	return appendBlueprintVersion(ctx, s.blueprints, b, v, baseVersion)
}
