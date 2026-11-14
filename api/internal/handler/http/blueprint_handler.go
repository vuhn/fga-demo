package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// BlueprintUsecases is everything the blueprint handler calls.
type BlueprintUsecases struct {
	Lister          usecase.BlueprintLister
	Getter          usecase.BlueprintGetter
	Drafter         usecase.BlueprintDrafter
	SketchDrafter   usecase.BlueprintSketchDrafter
	ImportPreviewer usecase.BlueprintDXFImportPreviewer
	Importer        usecase.BlueprintImporter
	OriginalGetter  usecase.BlueprintOriginalGetter
	OriginalExport  usecase.BlueprintOriginalExporter
	PlanEditor      usecase.BlueprintPlanEditor
	PlanConverter   usecase.BlueprintPlanConverter
	Creator         usecase.BlueprintCreator
	Reviser         usecase.BlueprintReviser
	ReviseStreamer  usecase.BlueprintReviseStreamer
	Reviewer        usecase.BlueprintReviewer
	BriefUpdater    usecase.BlueprintBriefUpdater
	VersionLister   usecase.BlueprintVersionLister
	Restorer        usecase.BlueprintVersionRestorer
	Renderer        usecase.BlueprintDrawingRenderer
	Renamer         usecase.BlueprintRenamer
	Deleter         usecase.BlueprintDeleter
	GrantLister     usecase.BlueprintGrantLister
	CandidateLister usecase.BlueprintShareCandidateLister
	Sharer          usecase.BlueprintSharer
	ShareRevoker    usecase.BlueprintShareRevoker
}

type BlueprintHandler struct{ uc BlueprintUsecases }

func NewBlueprintHandler(uc BlueprintUsecases) *BlueprintHandler { return &BlueprintHandler{uc: uc} }

func (h *BlueprintHandler) Register(r Routes) {
	g := r.Tenanted
	g.POST("/blueprint-drafts", h.draft)
	g.POST("/blueprint-drafts/sketch", h.draftFromSketch)
	g.POST("/blueprint-drafts/dxf-import", h.previewImport)
	g.POST("/blueprints/import", h.importDXF)
	g.GET("/blueprints/:id/source", h.source)
	g.GET("/blueprints/:id/source/edited", h.editedSource)
	g.POST("/blueprints/:id/plan-edits", h.planEdit)
	g.POST("/blueprints/:id/convert", h.convert)
	g.GET("/blueprints", h.list)
	g.POST("/blueprints", h.create)
	g.GET("/blueprints/:id", h.get)
	g.PATCH("/blueprints/:id", h.rename)
	g.DELETE("/blueprints/:id", h.delete)
	g.POST("/blueprints/:id/revisions", h.revise)
	g.POST("/blueprints/:id/revisions/stream", h.reviseStream)
	g.POST("/blueprints/:id/reviews", h.review)
	g.PUT("/blueprints/:id/brief", h.updateBrief)
	g.GET("/blueprints/:id/versions", h.listVersions)
	g.POST("/blueprints/:id/versions/:version/restore", h.restore)
	g.GET("/blueprints/:id/drawing", h.drawing)
	g.GET("/blueprints/:id/grants", h.listGrants)
	g.GET("/blueprints/:id/share-candidates", h.listShareCandidates)
	g.POST("/blueprints/:id/grants", h.share)
	g.DELETE("/blueprints/:id/grants", h.revoke)
}

func blueprintView(b entity.Blueprint) map[string]any {
	var source any
	if b.Source != nil {
		source = map[string]any{"name": b.Source.Name, "mime_type": b.Source.MimeType, "byte_size": b.Source.ByteSize}
	}
	return map[string]any{
		"id": b.ID, "tenant_id": b.TenantID, "project_id": b.ProjectID, "name": b.Name, "current_version": b.CurrentVersion,
		"source_kind": b.SourceKind, "source": source, "created_by": b.CreatedBy, "created_at": b.CreatedAt, "updated_at": b.UpdatedAt,
	}
}

func visibleBlueprintView(v entity.Visible[entity.Blueprint]) map[string]any {
	m := blueprintView(v.Value)
	m["permissions"] = v.Perms
	return m
}

// blueprintVersionSummary is a version without its JSON payloads.
func blueprintVersionSummary(v entity.BlueprintVersion) map[string]any {
	return map[string]any{
		"version": v.Version, "kind": v.Kind, "message": v.Message, "ok": v.OK, "created_by": v.CreatedBy, "created_at": v.CreatedAt,
	}
}

func blueprintVersionView(v entity.BlueprintVersion) map[string]any {
	m := blueprintVersionSummary(v)
	m["brief"], m["model"], m["checks"], m["notes"] = v.Brief, v.Model, v.Checks, v.Notes
	return m
}

func blueprintDetailView(d usecase.BlueprintDetail) map[string]any {
	m := blueprintView(d.Blueprint)
	if d.Perms != nil {
		m["permissions"] = d.Perms
	}
	m["version"] = blueprintVersionView(d.Version)
	return m
}

func draftView(d entity.BlueprintDraft) map[string]any {
	notes := d.Notes
	if notes == nil {
		notes = []string{}
	}
	return map[string]any{"brief": d.Brief, "notes": notes}
}

func (h *BlueprintHandler) draft(c *gin.Context) {
	var body struct {
		ProjectID string `json:"project_id"`
		Prompt    string `json:"prompt"`
	}
	if !bind(c, &body) {
		return
	}
	d, err := h.uc.Drafter.Draft(c, actor(c), tenant(c), body.ProjectID, body.Prompt)
	respond(c, http.StatusOK, draftView(d), err)
}

// draftFromSketch reads one multipart file, at most entity.MaxSketchBytes.
// The bytes go to the engine and are dropped; only name, type, and size are
// returned for the create call.
func (h *BlueprintHandler) draftFromSketch(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, entity.MaxSketchBytes+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		writeErr(c, entity.Invalid("file is required (PNG, JPG, PDF, or DXF, at most 10 MB)"))
		return
	}
	if fh.Size > entity.MaxSketchBytes {
		writeErr(c, entity.Invalid("file must be at most 10 MB"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		writeErr(c, err)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, entity.MaxSketchBytes+1))
	if err != nil {
		writeErr(c, err)
		return
	}
	mime := strings.TrimSpace(strings.Split(fh.Header.Get("Content-Type"), ";")[0])
	if mime == "" || mime == "application/octet-stream" {
		mime = strings.Split(http.DetectContentType(data), ";")[0]
	}
	if strings.EqualFold(filepath.Ext(fh.Filename), ".dxf") {
		mime = entity.DXFMimeType // browsers send "", octet-stream, or image/vnd.dxf
	}
	file := entity.Upload{Name: fh.Filename, MimeType: mime, Data: data}
	d, err := h.uc.SketchDrafter.DraftFromSketch(c, actor(c), tenant(c), c.PostForm("project_id"), file, c.PostForm("note"))
	if err != nil {
		writeErr(c, err)
		return
	}
	m := draftView(d)
	m["source"] = map[string]any{"name": fh.Filename, "mime_type": mime, "byte_size": len(data)}
	c.JSON(http.StatusOK, m)
}

// readDXF reads the multipart "file" of an import, at most entity.MaxImportBytes.
func readDXF(c *gin.Context) (entity.Upload, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, entity.MaxImportBytes+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		writeErr(c, entity.Invalid("file is required (a DXF, at most 20 MB)"))
		return entity.Upload{}, false
	}
	if fh.Size > entity.MaxImportBytes {
		writeErr(c, entity.Invalid("file must be at most 20 MB"))
		return entity.Upload{}, false
	}
	f, err := fh.Open()
	if err != nil {
		writeErr(c, err)
		return entity.Upload{}, false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, entity.MaxImportBytes+1))
	if err != nil {
		writeErr(c, err)
		return entity.Upload{}, false
	}
	// browsers send "", octet-stream or image/vnd.dxf for a DXF, so the extension decides
	if !strings.EqualFold(filepath.Ext(fh.Filename), ".dxf") {
		writeErr(c, entity.Invalid("only DXF files can be imported"))
		return entity.Upload{}, false
	}
	return entity.Upload{Name: filepath.Base(fh.Filename), MimeType: entity.DXFMimeType, Data: data}, true
}

// previewImport reads a DXF and returns its plans (each with the engine's drawing and checks) for the review
// step. Nothing is stored.
func (h *BlueprintHandler) previewImport(c *gin.Context) {
	file, ok := readDXF(c)
	if !ok {
		return
	}
	res, err := h.uc.ImportPreviewer.Preview(c, actor(c), tenant(c), c.PostForm("project_id"), file, c.PostForm("unit"))
	if err != nil {
		writeErr(c, err)
		return
	}
	plans := make([]map[string]any, 0, len(res.Plans))
	for _, p := range res.Plans {
		plans = append(plans, map[string]any{"index": p.Index, "name": p.Name, "room_count": p.RoomCount, "net_area": p.NetArea,
			"ok": p.OK, "checks": p.Checks, "svg": p.SVG})
	}
	c.JSON(http.StatusOK, map[string]any{"plans": plans, "report": res.Report,
		"source": map[string]any{"name": file.Name, "mime_type": file.MimeType, "byte_size": len(file.Data)}})
}

// importDXF creates a blueprint from one plan of a DXF and stores the original file.
func (h *BlueprintHandler) importDXF(c *gin.Context) {
	file, ok := readDXF(c)
	if !ok {
		return
	}
	plan := 1
	if v := c.PostForm("plan"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeErr(c, entity.Invalid("plan must be an integer"))
			return
		}
		plan = n
	}
	d, err := h.uc.Importer.Import(c, actor(c), tenant(c), usecase.ImportBlueprintInput{
		ProjectID: c.PostForm("project_id"), Name: c.PostForm("name"), File: file, Unit: c.PostForm("unit"), PlanIndex: plan})
	respond(c, http.StatusCreated, blueprintDetailView(d), err)
}

// source downloads the stored original of the current (or ?version=) version. can_edit only.
func (h *BlueprintHandler) source(c *gin.Context) {
	version := 0
	if v := c.Query("version"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeErr(c, entity.Invalid("version must be a positive integer"))
			return
		}
		version = n
	}
	o, err := h.uc.OriginalGetter.GetOriginal(c, actor(c), tenant(c), c.Param("id"), version)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": o.Name}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, no-store")
	c.Data(http.StatusOK, o.MimeType, o.Data)
}

// editedSource downloads the original file with the plan's edits written back into it. can_edit only.
// The report (applied, added, skipped) travels base64-encoded in X-Export-Report.
func (h *BlueprintHandler) editedSource(c *gin.Context) {
	version := 0
	if v := c.Query("version"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeErr(c, entity.Invalid("version must be a positive integer"))
			return
		}
		version = n
	}
	f, err := h.uc.OriginalExport.Export(c, actor(c), tenant(c), c.Param("id"), version)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Name}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Export-Report", base64.StdEncoding.EncodeToString(boundedReport(f.Report)))
	c.Data(http.StatusOK, f.MimeType, f.Data)
}

// boundedReport keeps the export report small enough for a response header: at most 20 lines per list
// (a proxy or browser drops headers of around 8 KB), with a last line counting the rest.
func boundedReport(raw json.RawMessage) json.RawMessage {
	var r map[string][]string
	if json.Unmarshal(raw, &r) != nil {
		return json.RawMessage(`{}`)
	}
	for k, v := range r {
		if len(v) > 20 {
			r[k] = append(append([]string{}, v[:20]...), fmt.Sprintf("… and %d more", len(v)-20))
		}
		for i, s := range r[k] {
			if rs := []rune(s); len(rs) > 200 {
				r[k][i] = string(rs[:200]) + "…"
			}
		}
	}
	out, _ := json.Marshal(r)
	return out
}

// planEdit applies explicit edits (no LLM) to an imported plan as a new version.
func (h *BlueprintHandler) planEdit(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10+4096)
	var body struct {
		Ops         json.RawMessage `json:"ops"`
		BaseVersion int             `json:"base_version"`
	}
	if !bind(c, &body) {
		return
	}
	d, err := h.uc.PlanEditor.Edit(c, actor(c), tenant(c), c.Param("id"), body.Ops, body.BaseVersion)
	respond(c, http.StatusCreated, blueprintDetailView(d), err)
}

// convert turns a blueprint made from a brief into a DXF plan (a new version, no brief, the drawing as its original).
func (h *BlueprintHandler) convert(c *gin.Context) {
	var body struct {
		BaseVersion int `json:"base_version"`
	}
	if !bind(c, &body) {
		return
	}
	d, err := h.uc.PlanConverter.Convert(c, actor(c), tenant(c), c.Param("id"), body.BaseVersion)
	respond(c, http.StatusCreated, blueprintDetailView(d), err)
}

func (h *BlueprintHandler) list(c *gin.Context) {
	bs, err := h.uc.Lister.List(c, actor(c), tenant(c), c.Query("project_id"))
	respond(c, http.StatusOK, mapSlice(bs, visibleBlueprintView), err)
}

func (h *BlueprintHandler) create(c *gin.Context) {
	var body struct {
		ProjectID  string          `json:"project_id"`
		Name       string          `json:"name"`
		Brief      json.RawMessage `json:"brief"`
		SourceKind string          `json:"source_kind"`
		Source     *struct {
			Name     string `json:"name"`
			MimeType string `json:"mime_type"`
			ByteSize int64  `json:"byte_size"`
		} `json:"source"`
		Prompt string `json:"prompt"`
	}
	if !bind(c, &body) {
		return
	}
	in := usecase.CreateBlueprintInput{ProjectID: body.ProjectID, Name: body.Name, Brief: body.Brief, SourceKind: body.SourceKind, Prompt: body.Prompt}
	if body.Source != nil {
		in.Source = &entity.BlueprintSource{Name: body.Source.Name, MimeType: body.Source.MimeType, ByteSize: body.Source.ByteSize}
	}
	d, err := h.uc.Creator.Create(c, actor(c), tenant(c), in)
	respond(c, http.StatusCreated, blueprintDetailView(d), err)
}

func (h *BlueprintHandler) get(c *gin.Context) {
	d, err := h.uc.Getter.Get(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, blueprintDetailView(d), err)
}

func (h *BlueprintHandler) rename(c *gin.Context) {
	var body struct{ Name string }
	if !bind(c, &body) {
		return
	}
	b, err := h.uc.Renamer.Rename(c, actor(c), tenant(c), c.Param("id"), body.Name)
	respond(c, http.StatusOK, blueprintView(b), err)
}

func (h *BlueprintHandler) delete(c *gin.Context) {
	noContent(c, h.uc.Deleter.Delete(c, actor(c), tenant(c), c.Param("id")))
}

func (h *BlueprintHandler) revise(c *gin.Context) {
	var body struct {
		Message     string `json:"message"`
		BaseVersion int    `json:"base_version"`
	}
	if !bind(c, &body) {
		return
	}
	d, err := h.uc.Reviser.Revise(c, actor(c), tenant(c), c.Param("id"), body.Message, body.BaseVersion)
	respond(c, http.StatusCreated, blueprintDetailView(d), err)
}

// reviseStream is revise answered as Server-Sent Events. The stream opens on
// the first progress event, which the service only emits after its Check and
// version test passed, so every refusal is still a plain JSON status. After
// that, success is a "done" event and failure an "error" event.
func (h *BlueprintHandler) reviseStream(c *gin.Context) {
	var body struct {
		Message     string `json:"message"`
		BaseVersion int    `json:"base_version"`
	}
	if !bind(c, &body) {
		return
	}
	started := false
	send := func(event string, v any) {
		if !started {
			started = true
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("X-Accel-Buffering", "no")
			c.Status(http.StatusOK)
		}
		data, _ := json.Marshal(v)
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
		c.Writer.Flush()
	}
	d, err := h.uc.ReviseStreamer.ReviseStream(c.Request.Context(), actor(c), tenant(c), c.Param("id"), body.Message, body.BaseVersion,
		func(p entity.BlueprintProgress) { send("progress", p) })
	if err != nil {
		if !started {
			writeErr(c, err)
			return
		}
		status, eb := errBody(c, err)
		eb["status"] = status
		send("error", eb)
		return
	}
	send("done", blueprintDetailView(d))
}

func (h *BlueprintHandler) review(c *gin.Context) {
	var body struct {
		BaseVersion int `json:"base_version"`
	}
	if !bind(c, &body) {
		return
	}
	d, err := h.uc.Reviewer.Review(c, actor(c), tenant(c), c.Param("id"), body.BaseVersion)
	respond(c, http.StatusCreated, blueprintDetailView(d), err)
}

func (h *BlueprintHandler) updateBrief(c *gin.Context) {
	var body struct {
		Brief       json.RawMessage `json:"brief"`
		BaseVersion int             `json:"base_version"`
	}
	if !bind(c, &body) {
		return
	}
	d, err := h.uc.BriefUpdater.UpdateBrief(c, actor(c), tenant(c), c.Param("id"), body.Brief, body.BaseVersion)
	respond(c, http.StatusCreated, blueprintDetailView(d), err)
}

func (h *BlueprintHandler) listVersions(c *gin.Context) {
	vs, err := h.uc.VersionLister.ListVersions(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(vs, blueprintVersionSummary), err)
}

func (h *BlueprintHandler) restore(c *gin.Context) {
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil {
		writeErr(c, entity.Invalid("version must be an integer"))
		return
	}
	var body struct {
		BaseVersion int `json:"base_version"`
	}
	if !bind(c, &body) {
		return
	}
	d, err := h.uc.Restorer.Restore(c, actor(c), tenant(c), c.Param("id"), version, body.BaseVersion)
	respond(c, http.StatusCreated, blueprintDetailView(d), err)
}

func (h *BlueprintHandler) drawing(c *gin.Context) {
	version := 0
	if v := c.Query("version"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeErr(c, entity.Invalid("version must be a positive integer"))
			return
		}
		version = n
	}
	format := c.DefaultQuery("format", entity.DrawingSVG)
	d, err := h.uc.Renderer.Render(c, actor(c), tenant(c), c.Param("id"), format, version)
	if err != nil {
		writeErr(c, err)
		return
	}
	if d.Format != entity.DrawingSVG {
		c.Header("Content-Disposition", `attachment; filename="`+d.Filename+`"`)
	}
	c.Header("Cache-Control", "private, no-store")
	c.Data(http.StatusOK, d.ContentType, d.Data)
}

func (h *BlueprintHandler) listGrants(c *gin.Context) {
	gs, err := h.uc.GrantLister.ListGrants(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(gs, grantView), err)
}

func (h *BlueprintHandler) listShareCandidates(c *gin.Context) {
	ms, err := h.uc.CandidateLister.ListShareCandidates(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(ms, memberView), err)
}

func (h *BlueprintHandler) share(c *gin.Context) {
	var body shareBody
	if !bind(c, &body) {
		return
	}
	g, err := h.uc.Sharer.Share(c, actor(c), tenant(c), c.Param("id"), body.input())
	respond(c, http.StatusCreated, grantView(g), err)
}

func (h *BlueprintHandler) revoke(c *gin.Context) {
	var body shareBody
	if !bind(c, &body) {
		return
	}
	noContent(c, h.uc.ShareRevoker.Revoke(c, actor(c), tenant(c), c.Param("id"), body.input()))
}
