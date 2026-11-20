package service

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

// loadBlueprint returns the blueprint when it exists in the header tenant.
func loadBlueprint(ctx context.Context, blueprints gateway.BlueprintGateway, tenantID, id string) (model.Blueprint, error) {
	if err := model.RequireUUID("id", id); err != nil {
		return model.Blueprint{}, err
	}
	b, err := blueprints.Get(ctx, id)
	if err != nil {
		return model.Blueprint{}, err
	}
	if b.TenantID != tenantID {
		return model.Blueprint{}, model.ErrNotFound
	}
	return b, nil
}

// requireBlueprintShare loads the blueprint in the tenant and requires can_share.
func requireBlueprintShare(ctx context.Context, blueprints gateway.BlueprintGateway, authz helper.Authorizer, actor, tenantID, id string) error {
	if _, err := loadBlueprint(ctx, blueprints, tenantID, id); err != nil {
		return err
	}
	return authz.Require(ctx, actor, "can_share", model.ObjectTypeBlueprint, id)
}

// requireBlueprintCreate loads the project in the tenant and requires
// can_create_blueprint on it.
func requireBlueprintCreate(ctx context.Context, projects gateway.ProjectGateway, authz helper.Authorizer, actor, tenantID, projectID string) error {
	if _, err := loadProject(ctx, projects, tenantID, projectID); err != nil {
		return err
	}
	return authz.Require(ctx, actor, "can_create_blueprint", model.ObjectTypeProject, projectID)
}

// loadBlueprintForEdit loads the blueprint, requires can_edit, and returns
// its current version when that is still baseVersion (409 otherwise). The
// engine is called only after this, so a denied or stale request never
// reaches the LLM.
func loadBlueprintForEdit(ctx context.Context, blueprints gateway.BlueprintGateway, authz helper.Authorizer, actor, tenantID, id string, baseVersion int) (model.Blueprint, model.BlueprintVersion, error) {
	b, err := loadBlueprint(ctx, blueprints, tenantID, id)
	if err != nil {
		return model.Blueprint{}, model.BlueprintVersion{}, err
	}
	if err := authz.Require(ctx, actor, "can_edit", model.ObjectTypeBlueprint, id); err != nil {
		return model.Blueprint{}, model.BlueprintVersion{}, err
	}
	if baseVersion < 1 {
		return model.Blueprint{}, model.BlueprintVersion{}, model.Invalid("base_version must be >= 1")
	}
	if baseVersion != b.CurrentVersion {
		return model.Blueprint{}, model.BlueprintVersion{}, staleVersion(b)
	}
	v, err := blueprints.GetVersion(ctx, id, b.CurrentVersion)
	return b, v, err
}

// isImported reports whether a version came from a CAD file (no brief).
func isImported(v model.BlueprintVersion) bool { return requireBrief(v) != nil }

// planOutcomeVersion is outcomeVersion for an imported plan: the brief stays null.
func planOutcomeVersion(blueprintID, kind, message, actor string, o model.BlueprintOutcome) model.BlueprintVersion {
	o.Brief = json.RawMessage("null")
	return outcomeVersion(blueprintID, kind, message, actor, o)
}

// requireBrief refuses the brief-based operations (AI review, form edit) on an imported plan,
// whose version has no brief: its geometry came from a file and is not regenerated.
func requireBrief(v model.BlueprintVersion) error {
	if t := strings.TrimSpace(string(v.Brief)); t == "" || t == "null" {
		return model.Invalid("this blueprint was imported from a DXF and has no brief; the AI review and the brief form are not available for it (use chat)")
	}
	return nil
}

func staleVersion(b model.Blueprint) error {
	return model.Conflict("blueprint is at version %d; reload and try again", b.CurrentVersion)
}

// prepareRevision is what a chat revision needs before the engine runs: the
// trimmed message, the blueprint, its current version, and the chat history.
// It does the can_edit Check and the base_version test, so a refused request
// never reaches the LLM.
func prepareRevision(ctx context.Context, blueprints gateway.BlueprintGateway, authz helper.Authorizer, actor, tenantID, id, message string, baseVersion int) (string, model.Blueprint, model.BlueprintVersion, []string, error) {
	message = strings.TrimSpace(message)
	if n := len([]rune(message)); n < 1 || n > 2000 {
		return "", model.Blueprint{}, model.BlueprintVersion{}, nil, model.Invalid("message length must be 1 to 2000")
	}
	b, cur, err := loadBlueprintForEdit(ctx, blueprints, authz, actor, tenantID, id, baseVersion)
	if err != nil {
		return "", model.Blueprint{}, model.BlueprintVersion{}, nil, err
	}
	history, err := chatHistory(ctx, blueprints, id)
	return message, b, cur, history, err
}

// requireBuilt turns an engine outcome without a model into a 400 carrying
// the engine's reasons. Nothing is stored for it.
func requireBuilt(o model.BlueprintOutcome) error {
	if o.Built() {
		return nil
	}
	if len(o.Issues) == 0 {
		return model.Invalid("the brief cannot be built")
	}
	return model.Invalid("%s", strings.Join(o.Issues, "; "))
}

// outcomeVersion is the version row for an engine outcome.
func outcomeVersion(blueprintID, kind, message, actor string, o model.BlueprintOutcome) model.BlueprintVersion {
	return model.BlueprintVersion{BlueprintID: blueprintID, Kind: kind, Message: message,
		Brief: o.Brief, Model: o.Model, Checks: o.Checks, Notes: append(o.Notes[:0:0], o.Notes...), OK: o.OK, CreatedBy: actor}
}

// appendBlueprintVersion stores v on top of base and returns the new detail.
func appendBlueprintVersion(ctx context.Context, blueprints gateway.BlueprintGateway, b model.Blueprint, v model.BlueprintVersion, base int) (blueprintDetail, error) {
	if err := blueprints.AppendVersion(ctx, v, base); err != nil {
		return blueprintDetail{}, err
	}
	b, err := blueprints.Get(ctx, b.ID)
	if err != nil {
		return blueprintDetail{}, err
	}
	v, err = blueprints.GetVersion(ctx, b.ID, base+1)
	return blueprintDetail{Blueprint: b, Version: v}, err
}

func blueprintID(b model.Blueprint) string { return b.ID }

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
		if (v.Kind == model.BlueprintVersionCreate || v.Kind == model.BlueprintVersionRevise) && strings.TrimSpace(v.Message) != "" {
			out = append(out, v.Message)
		}
	}
	slices.Reverse(out)
	return out, nil
}

// blueprintDetail is a blueprint with one of its versions. Perms is set only by the getter.
type blueprintDetail struct {
	Blueprint model.Blueprint
	Perms     model.Perms
	Version   model.BlueprintVersion
}
