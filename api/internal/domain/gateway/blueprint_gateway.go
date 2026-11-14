package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// BlueprintGateway is the blueprints and blueprint_versions display record.
type BlueprintGateway interface {
	// Create inserts the blueprint, its version 1, and its owner grant in one
	// transaction.
	Create(ctx context.Context, b entity.Blueprint, v entity.BlueprintVersion, owner entity.Grant) error
	Get(ctx context.Context, id string) (entity.Blueprint, error)
	// ListByIDs loads the given ids, restricted to the tenant and, when
	// projectID is not empty, to that project.
	ListByIDs(ctx context.Context, tenantID string, ids []string, projectID string) ([]entity.Blueprint, error)
	Rename(ctx context.Context, id, name string) (entity.Blueprint, error)
	// Delete removes the blueprint, its versions, and its grants in one transaction.
	Delete(ctx context.Context, id string) error
	GetVersion(ctx context.Context, id string, version int) (entity.BlueprintVersion, error)
	// ListVersions returns every version, newest first, without brief or model.
	ListVersions(ctx context.Context, id string) ([]entity.BlueprintVersion, error)
	// AppendVersion stores v as version base+1 and makes it current, only
	// while the current version is still base. Otherwise ErrConflict and
	// nothing is written.
	AppendVersion(ctx context.Context, v entity.BlueprintVersion, base int) error
}

// BlueprintGrantGateway is the resource_grants rows of blueprints.
type BlueprintGrantGateway interface {
	List(ctx context.Context, blueprintID string) ([]entity.Grant, error)
	// Upsert inserts the grant or replaces the relation of the existing grant
	// for the same subject, returning the previous grant when one existed.
	Upsert(ctx context.Context, g entity.Grant) (prev *entity.Grant, err error)
	Delete(ctx context.Context, id string) error
	// Restore puts a grant row back exactly, inserting or updating by id.
	Restore(ctx context.Context, g entity.Grant) error
}

// BlueprintRelationGateway builds the blueprint tuples. It is pure.
type BlueprintRelationGateway interface {
	// Created links the blueprint to its tenant, project, and owner.
	Created(b entity.Blueprint) []entity.Tuple
	// Grant validates a share on a blueprint and returns its tuple.
	Grant(g entity.Grant) (entity.Tuple, error)
}

// BlueprintEngine is the stateless blueprint-engine. It has no auth: callers
// run their Checks first. A request it rejects is ErrValidation; an engine or
// LLM that is down is ErrEngineUnavailable.
type BlueprintEngine interface {
	BriefFromText(ctx context.Context, prompt string) (entity.BlueprintDraft, error)
	BriefFromSketch(ctx context.Context, file entity.Upload, note string) (entity.BlueprintDraft, error)
	// Generate builds the brief. previous keeps unchanged rooms in place;
	// repair shrinks rooms toward their minimums when the brief is too big.
	Generate(ctx context.Context, brief, previous []byte, repair bool) (entity.BlueprintOutcome, error)
	// Revise turns a chat message into edits on the brief and rebuilds it.
	// history is the earlier chat messages, oldest first.
	Revise(ctx context.Context, brief, previous []byte, message string, history []string) (entity.BlueprintOutcome, error)
	// Review lets the engine's reviewer fix the brief's failed checks in up
	// to three rounds, keeping a round only when the issue score drops.
	Review(ctx context.Context, brief, previous []byte, history []string) (entity.BlueprintOutcome, error)
	Render(ctx context.Context, model []byte, format string, tb entity.BlueprintTitleBlock) (entity.Drawing, error)
}
