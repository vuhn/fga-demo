package entity

import (
	"encoding/json"
	"time"
)

// ObjectTypeBlueprint is the OpenFGA type and resource_grants.object_type of a blueprint.
const ObjectTypeBlueprint = "blueprint"

// Blueprint source kinds: how version 1's brief was produced.
const (
	BlueprintSourcePrompt = "prompt"
	BlueprintSourceSketch = "sketch"
	BlueprintSourceForm   = "form"
	BlueprintSourceImport = "import" // read from a DXF; no brief, geometry only
)

// Blueprint version kinds.
const (
	BlueprintVersionCreate  = "create"
	BlueprintVersionRevise  = "revise"
	BlueprintVersionReview  = "review"
	BlueprintVersionEdit    = "edit"
	BlueprintVersionRestore = "restore"
	BlueprintVersionImport  = "import"
)

// MaxImportBytes caps an imported CAD file. Unlike a sketch, the original is stored.
const MaxImportBytes = 20 << 20

// MaxSketchBytes caps an uploaded sketch. The file is sent to the engine and
// discarded; only its name, type, and size are kept.
const MaxSketchBytes = 10 << 20

// BlueprintSource describes an uploaded sketch for display.
type BlueprintSource struct {
	Name     string
	MimeType string
	ByteSize int64
}

// Blueprint lives directly in a project. Its content is the append-only
// versions; CurrentVersion is the newest.
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

// BlueprintVersion is one stored revision. Brief is the only editable source;
// Model, Checks, and Notes are what the engine derived from it. The JSON is
// opaque to Go: the engine owns its schema.
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

// BlueprintDraft is a brief extracted from a prompt or a sketch, not yet stored.
type BlueprintDraft struct {
	Brief json.RawMessage
	Notes []string
}

// BlueprintOutcome is one engine run: the brief it used (possibly shrunk),
// and the model when the brief could be built (nil otherwise, with Issues).
type BlueprintOutcome struct {
	Brief  json.RawMessage
	Model  json.RawMessage
	Checks json.RawMessage
	Issues []string
	OK     bool
	Notes  []string
}

// BlueprintProgress is one live step of an engine run (asking the model, a
// retry, rebuilding, checking) or a piece of the model's reasoning. It is shown while the run lasts, never stored.
type BlueprintProgress struct {
	Kind    string `json:"kind"` // "step", or "thinking" for a piece of the model's reasoning
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

// Built reports whether the engine produced a model.
func (o BlueprintOutcome) Built() bool {
	return len(o.Model) > 0 && string(o.Model) != "null"
}

// DXFMimeType is the canonical type for DXF uploads (browsers disagree).
const DXFMimeType = "image/vnd.dxf"

// Upload is a transient file sent to the engine.
type Upload struct {
	Name     string
	MimeType string
	Data     []byte
}

// BlueprintTitleBlock is the drawing sheet's title block.
type BlueprintTitleBlock struct {
	Project string
	Version int
	Issued  string
	Author  string
}

// Drawing formats.
const (
	DrawingSVG = "svg"
	DrawingPDF = "pdf"
	DrawingDXF = "dxf"
	DrawingIFC = "ifc"
)

// Drawing is rendered sheet bytes.
type Drawing struct {
	Format      string
	ContentType string
	Filename    string
	Data        []byte
}

// BlueprintOriginal is the stored file an imported blueprint version was read from.
type BlueprintOriginal struct {
	BlueprintID string
	Version     int
	Name        string
	MimeType    string
	ByteSize    int64
	SHA256      string
	Data        []byte
}

// BlueprintImportPlan is one plan the engine found in an imported file. Model and Checks are opaque JSON.
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

// BlueprintImport is what the engine read from a DXF: its plans and the import report (opaque JSON).
type BlueprintImport struct {
	Plans  []BlueprintImportPlan
	Report json.RawMessage
}
