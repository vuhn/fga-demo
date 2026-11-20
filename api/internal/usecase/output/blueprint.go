package output

import (
	"encoding/json"
	"time"
)

// Blueprint is the usecase output copy of a blueprint.
type Blueprint struct {
	ID             string
	TenantID       string
	ProjectID      string
	Name           string
	CurrentVersion int
	SourceKind     string
	Source         *BlueprintSource
	CreatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// BlueprintBriefUpdater is the result of BlueprintBriefUpdater.UpdateBrief.
type BlueprintBriefUpdater struct {
	BlueprintDetail
}

// BlueprintCreator is the result of BlueprintCreator.Create.
type BlueprintCreator struct {
	BlueprintDetail
}

// BlueprintDetail is a blueprint with one of its versions. Perms is set only
// by the getter.
type BlueprintDetail struct {
	Blueprint Blueprint
	Perms     map[string]bool
	Version   BlueprintVersion
}

// BlueprintDraft is the usecase output copy of a blueprintDraft.
type BlueprintDraft struct {
	Brief json.RawMessage
	Notes []string
}

// BlueprintDrafter is the result of BlueprintDrafter.Draft.
type BlueprintDrafter struct {
	BlueprintDraft
}

// BlueprintDrawingRenderer is the result of BlueprintDrawingRenderer.Render.
type BlueprintDrawingRenderer struct {
	Drawing
}

// BlueprintDXFImportPreviewer is the result of BlueprintDXFImportPreviewer.Preview.
type BlueprintDXFImportPreviewer struct {
	BlueprintImport
}

// BlueprintEstimate is the usecase output copy of a blueprintEstimate.
type BlueprintEstimate struct {
	ID          string
	BlueprintID string
	Version     int
	TotalLow    int64
	TotalBase   int64
	TotalHigh   int64
	Result      json.RawMessage
	CreatedAt   time.Time
}

// BlueprintGetter is the result of BlueprintGetter.Get.
type BlueprintGetter struct {
	BlueprintDetail
}

// BlueprintGrantLister is the result of BlueprintGrantLister.ListGrants.
type BlueprintGrantLister struct {
	Items []Grant
}

// BlueprintImport is the usecase output copy of a blueprintImport.
type BlueprintImport struct {
	Plans      []BlueprintImportPlan
	Report     json.RawMessage
	Conversion json.RawMessage
}

// BlueprintImportPlan is the usecase output copy of a blueprintImportPlan.
type BlueprintImportPlan struct {
	Index     int
	Name      string
	RoomCount int
	NetArea   float64
	OK        bool
	Model     json.RawMessage
	Checks    json.RawMessage
	SVG       string
}

// BlueprintImporter is the result of BlueprintImporter.Import.
type BlueprintImporter struct {
	BlueprintDetail
}

// BlueprintLister is the result of BlueprintLister.List.
type BlueprintLister struct {
	Items []Visible[Blueprint]
}

// BlueprintOriginal is the usecase output copy of a blueprintOriginal.
type BlueprintOriginal struct {
	BlueprintID string
	Version     int
	Name        string
	MimeType    string
	ByteSize    int64
	SHA256      string
	Data        []byte
}

// BlueprintOriginalExporter is the result of BlueprintOriginalExporter.Export.
type BlueprintOriginalExporter struct {
	PatchedFile
}

// BlueprintOriginalGetter is the result of BlueprintOriginalGetter.GetOriginal.
type BlueprintOriginalGetter struct {
	BlueprintOriginal
}

// BlueprintPlanConverter is the result of BlueprintPlanConverter.Convert.
type BlueprintPlanConverter struct {
	BlueprintDetail
}

// BlueprintPlanEditor is the result of BlueprintPlanEditor.Edit.
type BlueprintPlanEditor struct {
	BlueprintDetail
}

// BlueprintRenamer is the result of BlueprintRenamer.Rename.
type BlueprintRenamer struct {
	Blueprint
}

// BlueprintReviewer is the result of BlueprintReviewer.Review.
type BlueprintReviewer struct {
	BlueprintDetail
}

// BlueprintReviseStreamer is the result of BlueprintReviseStreamer.ReviseStream.
type BlueprintReviseStreamer struct {
	BlueprintDetail
}

// BlueprintReviser is the result of BlueprintReviser.Revise.
type BlueprintReviser struct {
	BlueprintDetail
}

// BlueprintShareCandidateLister is the result of BlueprintShareCandidateLister.ListShareCandidates.
type BlueprintShareCandidateLister struct {
	Items []Membership
}

// BlueprintSharer is the result of BlueprintSharer.Share.
type BlueprintSharer struct {
	Grant
}

// BlueprintSource is the usecase output copy of a blueprintSource.
type BlueprintSource struct {
	Name     string
	MimeType string
	ByteSize int64
}

// BlueprintVersion is the usecase output copy of a blueprintVersion.
type BlueprintVersion struct {
	BlueprintID string
	Version     int
	Kind        string
	Message     string
	Brief       json.RawMessage
	Model       json.RawMessage
	Checks      json.RawMessage
	Notes       []string
	OK          bool
	CreatedBy   string
	CreatedAt   time.Time
}

// BlueprintVersionLister is the result of BlueprintVersionLister.ListVersions.
type BlueprintVersionLister struct {
	Items []BlueprintVersion
}

// BlueprintVersionRestorer is the result of BlueprintVersionRestorer.Restore.
type BlueprintVersionRestorer struct {
	BlueprintDetail
}

// Drawing is the usecase output copy of a drawing.
type Drawing struct {
	Format      string
	ContentType string
	Filename    string
	Data        []byte
}

// PatchedFile is the usecase output copy of a patchedFile.
type PatchedFile struct {
	Name     string
	MimeType string
	Data     []byte
	Report   json.RawMessage
}
