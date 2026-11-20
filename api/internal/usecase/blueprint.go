package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type BlueprintLister interface {
	// List returns every visible blueprint in the tenant, or only those in
	// projectID when it is not empty.
	List(ctx context.Context, in input.BlueprintLister) (output.BlueprintLister, error)
}

type BlueprintGetter interface {
	Get(ctx context.Context, in input.BlueprintGetter) (output.BlueprintGetter, error)
}

// BlueprintDrafter extracts a brief from a prompt. It writes nothing.
type BlueprintDrafter interface {
	Draft(ctx context.Context, in input.BlueprintDrafter) (output.BlueprintDrafter, error)
}

type BlueprintCreator interface {
	Create(ctx context.Context, in input.BlueprintCreator) (output.BlueprintCreator, error)
}

// BlueprintReviser applies a chat message to the current brief.
type BlueprintReviser interface {
	Revise(ctx context.Context, in input.BlueprintReviser) (output.BlueprintReviser, error)
}

// BlueprintReviseStreamer is BlueprintReviser that reports live progress.
// Every refusal (Check, stale base_version, validation, engine down) is
// returned before the first onProgress call, so the caller can still answer
// with a plain status; afterwards errors arrive as the returned error.
type BlueprintReviseStreamer interface {
	ReviseStream(ctx context.Context, in input.BlueprintReviseStreamer, onProgress func(model.BlueprintProgress)) (output.BlueprintReviseStreamer, error)
}

// BlueprintReviewer lets the engine's reviewer fix the current version's
// failed checks ("Fix issues with AI").
type BlueprintReviewer interface {
	Review(ctx context.Context, in input.BlueprintReviewer) (output.BlueprintReviewer, error)
}

// BlueprintBriefUpdater replaces the brief with a form edit.
type BlueprintBriefUpdater interface {
	UpdateBrief(ctx context.Context, in input.BlueprintBriefUpdater) (output.BlueprintBriefUpdater, error)
}

type BlueprintVersionLister interface {
	ListVersions(ctx context.Context, in input.BlueprintVersionLister) (output.BlueprintVersionLister, error)
}

// BlueprintVersionRestorer copies an older version into a new one.
type BlueprintVersionRestorer interface {
	Restore(ctx context.Context, in input.BlueprintVersionRestorer) (output.BlueprintVersionRestorer, error)
}

// BlueprintDrawingRenderer renders the current version, or version when > 0.
type BlueprintDrawingRenderer interface {
	Render(ctx context.Context, in input.BlueprintDrawingRenderer) (output.BlueprintDrawingRenderer, error)
}

type BlueprintRenamer interface {
	Rename(ctx context.Context, in input.BlueprintRenamer) (output.BlueprintRenamer, error)
}

type BlueprintDeleter interface {
	Delete(ctx context.Context, in input.BlueprintDeleter) error
}

type BlueprintGrantLister interface {
	ListGrants(ctx context.Context, in input.BlueprintGrantLister) (output.BlueprintGrantLister, error)
}

// BlueprintShareCandidateLister lists the external members a sharer can pick.
type BlueprintShareCandidateLister interface {
	ListShareCandidates(ctx context.Context, in input.BlueprintShareCandidateLister) (output.BlueprintShareCandidateLister, error)
}

type BlueprintSharer interface {
	Share(ctx context.Context, in input.BlueprintSharer) (output.BlueprintSharer, error)
}

type BlueprintShareRevoker interface {
	Revoke(ctx context.Context, in input.BlueprintShareRevoker) error
}

// BlueprintDXFImportPreviewer reads a DXF (or a PDF/image, converted to one first) and returns the plans found, for the review step. Nothing is stored.
type BlueprintDXFImportPreviewer interface {
	Preview(ctx context.Context, in input.BlueprintDXFImportPreviewer) (output.BlueprintDXFImportPreviewer, error)
}

// BlueprintImporter creates a blueprint from one plan of a DXF and keeps the original file.
type BlueprintImporter interface {
	Import(ctx context.Context, in input.BlueprintImporter) (output.BlueprintImporter, error)
}

// BlueprintPlanEditor applies explicit edits (no LLM) to an imported plan as a new version.
type BlueprintPlanEditor interface {
	Edit(ctx context.Context, in input.BlueprintPlanEditor) (output.BlueprintPlanEditor, error)
}

// BlueprintPlanConverter turns a blueprint made from a brief (before DXF plans) into a DXF plan: the engine
// draws its plan into a DXF, which is stored as the original of a new version without a brief.
type BlueprintPlanConverter interface {
	Convert(ctx context.Context, in input.BlueprintPlanConverter) (output.BlueprintPlanConverter, error)
}

// BlueprintOriginalExporter returns the original file of an imported plan with the edits of a version
// (0 = current) written back into it.
type BlueprintOriginalExporter interface {
	Export(ctx context.Context, in input.BlueprintOriginalExporter) (output.BlueprintOriginalExporter, error)
}

// BlueprintOriginalGetter returns the stored original of the current (or a given) version.
type BlueprintOriginalGetter interface {
	GetOriginal(ctx context.Context, in input.BlueprintOriginalGetter) (output.BlueprintOriginalGetter, error)
}
