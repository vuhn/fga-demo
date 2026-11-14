package http

import (
	"encoding/json"
	"io"
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
	Creator         usecase.BlueprintCreator
	Reviser         usecase.BlueprintReviser
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
	g.GET("/blueprints", h.list)
	g.POST("/blueprints", h.create)
	g.GET("/blueprints/:id", h.get)
	g.PATCH("/blueprints/:id", h.rename)
	g.DELETE("/blueprints/:id", h.delete)
	g.POST("/blueprints/:id/revisions", h.revise)
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
