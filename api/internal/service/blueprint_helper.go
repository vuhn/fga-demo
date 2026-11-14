package service

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// loadBlueprint returns the blueprint when it exists in the header tenant.
func loadBlueprint(ctx context.Context, blueprints gateway.BlueprintGateway, tenantID, id string) (entity.Blueprint, error) {
	if err := entity.RequireUUID("id", id); err != nil {
		return entity.Blueprint{}, err
	}
	b, err := blueprints.Get(ctx, id)
	if err != nil {
		return entity.Blueprint{}, err
	}
	if b.TenantID != tenantID {
		return entity.Blueprint{}, entity.ErrNotFound
	}
	return b, nil
}

// requireBlueprintShare loads the blueprint in the tenant and requires can_share.
func requireBlueprintShare(ctx context.Context, blueprints gateway.BlueprintGateway, authz usecase.Authorizer, actor, tenantID, id string) error {
	if _, err := loadBlueprint(ctx, blueprints, tenantID, id); err != nil {
		return err
	}
	return authz.Require(ctx, actor, "can_share", entity.ObjectTypeBlueprint, id)
}

// requireBlueprintCreate loads the project in the tenant and requires
// can_create_blueprint on it.
func requireBlueprintCreate(ctx context.Context, projects gateway.ProjectGateway, authz usecase.Authorizer, actor, tenantID, projectID string) error {
	if _, err := loadProject(ctx, projects, tenantID, projectID); err != nil {
		return err
	}
	return authz.Require(ctx, actor, "can_create_blueprint", entity.ObjectTypeProject, projectID)
}

// loadBlueprintForEdit loads the blueprint, requires can_edit, and returns
// its current version when that is still baseVersion (409 otherwise). The
// engine is called only after this, so a denied or stale request never
// reaches the LLM.
func loadBlueprintForEdit(ctx context.Context, blueprints gateway.BlueprintGateway, authz usecase.Authorizer, actor, tenantID, id string, baseVersion int) (entity.Blueprint, entity.BlueprintVersion, error) {
	b, err := loadBlueprint(ctx, blueprints, tenantID, id)
	if err != nil {
		return entity.Blueprint{}, entity.BlueprintVersion{}, err
	}
	if err := authz.Require(ctx, actor, "can_edit", entity.ObjectTypeBlueprint, id); err != nil {
		return entity.Blueprint{}, entity.BlueprintVersion{}, err
	}
	if baseVersion < 1 {
		return entity.Blueprint{}, entity.BlueprintVersion{}, entity.Invalid("base_version must be >= 1")
	}
	if baseVersion != b.CurrentVersion {
		return entity.Blueprint{}, entity.BlueprintVersion{}, staleVersion(b)
	}
	v, err := blueprints.GetVersion(ctx, id, b.CurrentVersion)
	return b, v, err
}

func staleVersion(b entity.Blueprint) error {
	return entity.Conflict("blueprint is at version %d; reload and try again", b.CurrentVersion)
}

// requireBuilt turns an engine outcome without a model into a 400 carrying
// the engine's reasons. Nothing is stored for it.
func requireBuilt(o entity.BlueprintOutcome) error {
	if o.Built() {
		return nil
	}
	if len(o.Issues) == 0 {
		return entity.Invalid("the brief cannot be built")
	}
	return entity.Invalid("%s", strings.Join(o.Issues, "; "))
}

// outcomeVersion is the version row for an engine outcome.
func outcomeVersion(blueprintID, kind, message, actor string, o entity.BlueprintOutcome) entity.BlueprintVersion {
	return entity.BlueprintVersion{BlueprintID: blueprintID, Kind: kind, Message: message,
		Brief: o.Brief, Model: o.Model, Checks: o.Checks, Notes: append(o.Notes[:0:0], o.Notes...), OK: o.OK, CreatedBy: actor}
}

// appendBlueprintVersion stores v on top of base and returns the new detail.
func appendBlueprintVersion(ctx context.Context, blueprints gateway.BlueprintGateway, b entity.Blueprint, v entity.BlueprintVersion, base int) (usecase.BlueprintDetail, error) {
	if err := blueprints.AppendVersion(ctx, v, base); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	b, err := blueprints.Get(ctx, b.ID)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	v, err = blueprints.GetVersion(ctx, b.ID, base+1)
	return usecase.BlueprintDetail{Blueprint: b, Version: v}, err
}

func blueprintID(b entity.Blueprint) string { return b.ID }

// sameJSON reports whether two JSON documents hold the same value
// (key order and spacing aside).
func sameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// maxChatHistory is how many earlier chat messages go to the engine.
const maxChatHistory = 6

// chatHistory is the messages of the last create and revise versions, oldest
// first: the engine's chat memory. It is read from the display record only.
func chatHistory(ctx context.Context, blueprints gateway.BlueprintGateway, id string) ([]string, error) {
	vs, err := blueprints.ListVersions(ctx, id) // newest first
	if err != nil {
		return nil, err
	}
	var out []string
	for _, v := range vs {
		if len(out) == maxChatHistory {
			break
		}
		if (v.Kind == entity.BlueprintVersionCreate || v.Kind == entity.BlueprintVersionRevise) && strings.TrimSpace(v.Message) != "" {
			out = append(out, v.Message)
		}
	}
	slices.Reverse(out)
	return out, nil
}
