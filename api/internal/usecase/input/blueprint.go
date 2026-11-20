package input

import (
	"encoding/json"
)

// BlueprintBriefUpdater is the input of BlueprintBriefUpdater.UpdateBrief.
type BlueprintBriefUpdater struct {
	Actor       string
	TenantID    string
	ID          string
	Brief       json.RawMessage
	BaseVersion int
}

// BlueprintCreator is the input of BlueprintCreator.Create.
type BlueprintCreator struct {
	Actor      string
	TenantID   string
	ProjectID  string
	Name       string
	Brief      json.RawMessage
	SourceKind string
	Prompt     string
}

// BlueprintDeleter is the input of BlueprintDeleter.Delete.
type BlueprintDeleter struct {
	Actor    string
	TenantID string
	ID       string
}

// BlueprintDrafter is the input of BlueprintDrafter.Draft.
type BlueprintDrafter struct {
	Actor     string
	TenantID  string
	ProjectID string
	Prompt    string
}

// BlueprintDrawingRenderer is the input of BlueprintDrawingRenderer.Render.
type BlueprintDrawingRenderer struct {
	Actor    string
	TenantID string
	ID       string
	Format   string
	Version  int
}

// BlueprintDXFImportPreviewer is the input of BlueprintDXFImportPreviewer.Preview.
type BlueprintDXFImportPreviewer struct {
	Actor     string
	TenantID  string
	ProjectID string
	File      Upload
	Unit      string
	Opts      ConvertOptions
}

// BlueprintGetter is the input of BlueprintGetter.Get.
type BlueprintGetter struct {
	Actor    string
	TenantID string
	ID       string
}

// BlueprintGrantLister is the input of BlueprintGrantLister.ListGrants.
type BlueprintGrantLister struct {
	Actor    string
	TenantID string
	ID       string
}

// BlueprintImporter is the input of BlueprintImporter.Import.
type BlueprintImporter struct {
	Actor     string
	TenantID  string
	ProjectID string
	Name      string
	File      Upload
	Unit      string
	Convert   ConvertOptions
	PlanIndex int
}

// BlueprintLister is the input of BlueprintLister.List.
type BlueprintLister struct {
	Actor     string
	TenantID  string
	ProjectID string
}

// BlueprintOriginalExporter is the input of BlueprintOriginalExporter.Export.
type BlueprintOriginalExporter struct {
	Actor    string
	TenantID string
	ID       string
	Version  int
}

// BlueprintOriginalGetter is the input of BlueprintOriginalGetter.GetOriginal.
type BlueprintOriginalGetter struct {
	Actor    string
	TenantID string
	ID       string
	Version  int
}

// BlueprintPlanConverter is the input of BlueprintPlanConverter.Convert.
type BlueprintPlanConverter struct {
	Actor       string
	TenantID    string
	ID          string
	BaseVersion int
}

// BlueprintPlanEditor is the input of BlueprintPlanEditor.Edit.
type BlueprintPlanEditor struct {
	Actor       string
	TenantID    string
	ID          string
	Ops         json.RawMessage
	BaseVersion int
}

// BlueprintRenamer is the input of BlueprintRenamer.Rename.
type BlueprintRenamer struct {
	Actor    string
	TenantID string
	ID       string
	Name     string
}

// BlueprintReviewer is the input of BlueprintReviewer.Review.
type BlueprintReviewer struct {
	Actor       string
	TenantID    string
	ID          string
	BaseVersion int
}

// BlueprintReviseStreamer is the input of BlueprintReviseStreamer.ReviseStream.
type BlueprintReviseStreamer struct {
	Actor       string
	TenantID    string
	ID          string
	Message     string
	BaseVersion int
}

// BlueprintReviser is the input of BlueprintReviser.Revise.
type BlueprintReviser struct {
	Actor       string
	TenantID    string
	ID          string
	Message     string
	BaseVersion int
}

// BlueprintShareCandidateLister is the input of BlueprintShareCandidateLister.ListShareCandidates.
type BlueprintShareCandidateLister struct {
	Actor    string
	TenantID string
	ID       string
}

// BlueprintShareRevoker is the input of BlueprintShareRevoker.Revoke.
type BlueprintShareRevoker struct {
	Actor    string
	TenantID string
	ID       string
	Relation string
	Subject  string
	UserID   string
}

// BlueprintSharer is the input of BlueprintSharer.Share.
type BlueprintSharer struct {
	Actor    string
	TenantID string
	ID       string
	Relation string
	Subject  string
	UserID   string
}

// BlueprintVersionLister is the input of BlueprintVersionLister.ListVersions.
type BlueprintVersionLister struct {
	Actor    string
	TenantID string
	ID       string
}

// BlueprintVersionRestorer is the input of BlueprintVersionRestorer.Restore.
type BlueprintVersionRestorer struct {
	Actor       string
	TenantID    string
	ID          string
	Version     int
	BaseVersion int
}

// ConvertOptions tell the engine how to read a PDF or picture.
type ConvertOptions struct {
	Scale    int
	LongestM float64
}

// Upload is a file sent with a request; the engine reads it and nothing keeps it.
type Upload struct {
	Name     string
	MimeType string
	Data     []byte
}
