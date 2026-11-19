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
	// CreateImported is Create plus the original file, in the same transaction. The blueprint's own Source is
	// what is recorded as the source (the original file for an import, the sketch for a created plan, or none).
	CreateImported(ctx context.Context, b entity.Blueprint, v entity.BlueprintVersion, owner entity.Grant, original entity.BlueprintOriginal) error
	// GetOriginal returns the stored file of a version; ErrNotFound when it has none.
	GetOriginal(ctx context.Context, id string, version int) (entity.BlueprintOriginal, error)
	// FindOriginal returns the newest stored file of the blueprint (the one its edits are written back into);
	// ErrNotFound when it has none.
	FindOriginal(ctx context.Context, id string) (entity.BlueprintOriginal, error)
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
	// AppendVersionWithOriginal is AppendVersion plus the original file stored with the new version, in the
	// same transaction (converting a plan made from a brief into a DXF plan).
	AppendVersionWithOriginal(ctx context.Context, v entity.BlueprintVersion, base int, original entity.BlueprintOriginal) error
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

// BlueprintAgent is the blueprint-engine's LLM operations: everything that
// asks a model to read or change a blueprint. It has no auth: callers run
// their Checks first. A request it rejects is ErrValidation; an agent or LLM
// that is down is ErrEngineUnavailable.
type BlueprintAgent interface {
	BriefFromText(ctx context.Context, prompt string) (entity.BlueprintDraft, error)
	// Revise turns a chat message into edits on the brief and rebuilds it.
	// history is the earlier chat messages, oldest first.
	Revise(ctx context.Context, brief, previous []byte, message string, history []string) (entity.BlueprintOutcome, error)
	// ReviseStream is Revise that reports each step to onProgress while it
	// works. An error returned before the first callback is the same as
	// Revise's (the caller can still answer with a plain status).
	ReviseStream(ctx context.Context, brief, previous []byte, message string, history []string, onProgress func(entity.BlueprintProgress)) (entity.BlueprintOutcome, error)
	// RevisePlan and RevisePlanStream are the same for an imported plan, which has no brief: the message
	// becomes edits on the stored model (rename, fixtures, door and window widths).
	RevisePlan(ctx context.Context, model []byte, message string, history []string) (entity.BlueprintOutcome, error)
	RevisePlanStream(ctx context.Context, model []byte, message string, history []string, onProgress func(entity.BlueprintProgress)) (entity.BlueprintOutcome, error)
	// Review lets the reviewer fix the brief's failed checks in up
	// to three rounds, keeping a round only when the issue score drops.
	Review(ctx context.Context, brief, previous []byte, history []string) (entity.BlueprintOutcome, error)
}

// BlueprintEngine is the blueprint-engine's deterministic operations: layout,
// DXF import, conversion, editing and drawing. No LLM. It has no auth: callers
// run their Checks first. A request it rejects is ErrValidation; an engine
// that is down is ErrEngineUnavailable.
type BlueprintEngine interface {
	// Generate builds the brief. previous keeps unchanged rooms in place;
	// repair shrinks rooms toward their minimums when the brief is too big.
	Generate(ctx context.Context, brief, previous []byte, repair bool) (entity.BlueprintOutcome, error)
	// CreatePlan builds the brief into a plan the way an import does: the outcome has no brief, the model as read
	// back from a DXF the engine drew (outcome.DXF), so the plan is edited and exported like an imported one.
	CreatePlan(ctx context.Context, brief []byte) (entity.BlueprintOutcome, error)
	// EditPlan applies explicit operations (JSON, no LLM) to an imported plan's model.
	EditPlan(ctx context.Context, model, ops []byte) (entity.BlueprintOutcome, error)
	// PatchDXF writes the differences between base (the plan as imported) and current into the original file.
	PatchDXF(ctx context.Context, original entity.Upload, base, current []byte) (entity.PatchedFile, error)
	// DrawPlan draws a plan made from a brief (its stored model) into a DXF and reads it back, so it becomes a
	// plan with an original and no brief. title is the blueprint's name. No LLM.
	DrawPlan(ctx context.Context, model []byte, title string) (entity.BlueprintOutcome, error)
	// ConvertToDXF turns a PDF or image floor plan into a DXF (no LLM). Deterministic for the same file and options.
	ConvertToDXF(ctx context.Context, file entity.Upload, opts entity.ConvertOptions) (entity.BlueprintConversion, error)
	// ImportDXF reads the plans out of a CAD file. No LLM; the same file always gives the same plans.
	ImportDXF(ctx context.Context, file entity.Upload, unit string) (entity.BlueprintImport, error)
	Render(ctx context.Context, model []byte, format string, tb entity.BlueprintTitleBlock) (entity.Drawing, error)
}
