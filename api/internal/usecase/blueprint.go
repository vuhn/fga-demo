package usecase

import (
	"context"
	"encoding/json"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// BlueprintDetail is a blueprint with one of its versions. Perms is set only
// by the getter.
type BlueprintDetail struct {
	Blueprint model.Blueprint
	Perms     model.Perms
	Version   model.BlueprintVersion
}

type BlueprintLister interface {
	// List returns every visible blueprint in the tenant, or only those in
	// projectID when it is not empty.
	List(ctx context.Context, actor, tenantID, projectID string) ([]model.Visible[model.Blueprint], error)
}

type BlueprintGetter interface {
	Get(ctx context.Context, actor, tenantID, id string) (BlueprintDetail, error)
}

// BlueprintDrafter extracts a brief from a prompt. It writes nothing.
type BlueprintDrafter interface {
	Draft(ctx context.Context, actor, tenantID, projectID, prompt string) (model.BlueprintDraft, error)
}

type CreateBlueprintInput struct {
	ProjectID  string
	Name       string
	Brief      json.RawMessage
	SourceKind string
	// Prompt is the description or description the brief was drafted from; it
	// becomes version 1's message, the start of the chat history.
	Prompt string
}

type BlueprintCreator interface {
	Create(ctx context.Context, actor, tenantID string, in CreateBlueprintInput) (BlueprintDetail, error)
}

// BlueprintReviser applies a chat message to the current brief.
type BlueprintReviser interface {
	Revise(ctx context.Context, actor, tenantID, id, message string, baseVersion int) (BlueprintDetail, error)
}

// BlueprintReviseStreamer is BlueprintReviser that reports live progress.
// Every refusal (Check, stale base_version, validation, engine down) is
// returned before the first onProgress call, so the caller can still answer
// with a plain status; afterwards errors arrive as the returned error.
type BlueprintReviseStreamer interface {
	ReviseStream(ctx context.Context, actor, tenantID, id, message string, baseVersion int, onProgress func(model.BlueprintProgress)) (BlueprintDetail, error)
}

// BlueprintReviewer lets the engine's reviewer fix the current version's
// failed checks ("Fix issues with AI").
type BlueprintReviewer interface {
	Review(ctx context.Context, actor, tenantID, id string, baseVersion int) (BlueprintDetail, error)
}

// BlueprintBriefUpdater replaces the brief with a form edit.
type BlueprintBriefUpdater interface {
	UpdateBrief(ctx context.Context, actor, tenantID, id string, brief json.RawMessage, baseVersion int) (BlueprintDetail, error)
}

type BlueprintVersionLister interface {
	ListVersions(ctx context.Context, actor, tenantID, id string) ([]model.BlueprintVersion, error)
}

// BlueprintVersionRestorer copies an older version into a new one.
type BlueprintVersionRestorer interface {
	Restore(ctx context.Context, actor, tenantID, id string, version, baseVersion int) (BlueprintDetail, error)
}

// BlueprintDrawingRenderer renders the current version, or version when > 0.
type BlueprintDrawingRenderer interface {
	Render(ctx context.Context, actor, tenantID, id, format string, version int) (model.Drawing, error)
}

type BlueprintRenamer interface {
	Rename(ctx context.Context, actor, tenantID, id, name string) (model.Blueprint, error)
}

type BlueprintDeleter interface {
	Delete(ctx context.Context, actor, tenantID, id string) error
}

type BlueprintGrantLister interface {
	ListGrants(ctx context.Context, actor, tenantID, id string) ([]model.Grant, error)
}

// BlueprintShareCandidateLister lists the external members a sharer can pick.
type BlueprintShareCandidateLister interface {
	ListShareCandidates(ctx context.Context, actor, tenantID, id string) ([]model.Membership, error)
}

type BlueprintSharer interface {
	Share(ctx context.Context, actor, tenantID, id string, in model.ShareInput) (model.Grant, error)
}

type BlueprintShareRevoker interface {
	Revoke(ctx context.Context, actor, tenantID, id string, in model.ShareInput) error
}

// BlueprintDXFImportPreviewer reads a DXF (or a PDF/image, converted to one first) and returns the plans found, for the review step. Nothing is stored.
type BlueprintDXFImportPreviewer interface {
	Preview(ctx context.Context, actor, tenantID, projectID string, file model.Upload, unit string, opts model.ConvertOptions) (model.BlueprintImport, error)
}

type ImportBlueprintInput struct {
	ProjectID string
	Name      string
	File      model.Upload
	Unit      string
	Convert   model.ConvertOptions // for a PDF or image
	PlanIndex int                  // 1-based, as the preview numbered them
}

// BlueprintImporter creates a blueprint from one plan of a DXF and keeps the original file.
type BlueprintImporter interface {
	Import(ctx context.Context, actor, tenantID string, in ImportBlueprintInput) (BlueprintDetail, error)
}

// BlueprintPlanEditor applies explicit edits (no LLM) to an imported plan as a new version.
type BlueprintPlanEditor interface {
	Edit(ctx context.Context, actor, tenantID, id string, ops json.RawMessage, baseVersion int) (BlueprintDetail, error)
}

// BlueprintPlanConverter turns a blueprint made from a brief (before DXF plans) into a DXF plan: the engine
// draws its plan into a DXF, which is stored as the original of a new version without a brief.
type BlueprintPlanConverter interface {
	Convert(ctx context.Context, actor, tenantID, id string, baseVersion int) (BlueprintDetail, error)
}

// BlueprintOriginalExporter returns the original file of an imported plan with the edits of a version
// (0 = current) written back into it.
type BlueprintOriginalExporter interface {
	Export(ctx context.Context, actor, tenantID, id string, version int) (model.PatchedFile, error)
}

// BlueprintOriginalGetter returns the stored original of the current (or a given) version.
type BlueprintOriginalGetter interface {
	GetOriginal(ctx context.Context, actor, tenantID, id string, version int) (model.BlueprintOriginal, error)
}
