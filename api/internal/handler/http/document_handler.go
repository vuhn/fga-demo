package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// DocumentUsecases is everything the document handler calls.
type DocumentUsecases struct {
	Lister          usecase.DocumentLister
	Getter          usecase.DocumentGetter
	Creator         usecase.DocumentCreator
	Renamer         usecase.DocumentRenamer
	Deleter         usecase.DocumentDeleter
	GrantLister     usecase.DocumentGrantLister
	CandidateLister usecase.DocumentShareCandidateLister
	Sharer          usecase.DocumentSharer
	ShareRevoker    usecase.DocumentShareRevoker
}

type DocumentHandler struct{ uc DocumentUsecases }

func NewDocumentHandler(uc DocumentUsecases) *DocumentHandler { return &DocumentHandler{uc: uc} }

func (h *DocumentHandler) Register(r Routes) {
	g := r.Tenanted
	g.GET("/documents", h.list)
	g.POST("/documents", h.create)
	g.GET("/documents/:id", h.get)
	g.PATCH("/documents/:id", h.rename)
	g.DELETE("/documents/:id", h.delete)
	g.GET("/documents/:id/grants", h.listGrants)
	g.GET("/documents/:id/share-candidates", h.listShareCandidates)
	g.POST("/documents/:id/grants", h.share)
	g.DELETE("/documents/:id/grants", h.revoke)
}

func documentView(d entity.Document) map[string]any {
	return map[string]any{
		"id": d.ID, "tenant_id": d.TenantID, "folder_id": d.FolderID, "name": d.Name,
		"mime_type": d.MimeType, "byte_size": d.ByteSize, "created_by": d.CreatedBy, "created_at": d.CreatedAt,
	}
}

func visibleDocumentView(v entity.Visible[entity.Document]) map[string]any {
	m := documentView(v.Value)
	m["permissions"] = v.Perms
	return m
}

func (h *DocumentHandler) list(c *gin.Context) {
	ds, err := h.uc.Lister.List(c, actor(c), tenant(c), c.Query("folder_id"))
	respond(c, http.StatusOK, mapSlice(ds, visibleDocumentView), err)
}

func (h *DocumentHandler) create(c *gin.Context) {
	var body map[string]any
	if !bind(c, &body) {
		return
	}
	in := usecase.CreateDocumentInput{}
	in.FolderID, _ = body["folder_id"].(string)
	in.Name, _ = body["name"].(string)
	in.MimeType, _ = body["mime_type"].(string)
	size, ok := body["byte_size"].(float64)
	if !ok || size != float64(int64(size)) {
		writeErr(c, entity.Invalid("byte_size must be an integer"))
		return
	}
	in.ByteSize = int64(size)
	d, err := h.uc.Creator.Create(c, actor(c), tenant(c), in)
	respond(c, http.StatusCreated, documentView(d), err)
}

func (h *DocumentHandler) get(c *gin.Context) {
	d, err := h.uc.Getter.Get(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, visibleDocumentView(d), err)
}

func (h *DocumentHandler) rename(c *gin.Context) {
	var body struct{ Name string }
	if !bind(c, &body) {
		return
	}
	d, err := h.uc.Renamer.Rename(c, actor(c), tenant(c), c.Param("id"), body.Name)
	respond(c, http.StatusOK, documentView(d), err)
}

func (h *DocumentHandler) delete(c *gin.Context) {
	noContent(c, h.uc.Deleter.Delete(c, actor(c), tenant(c), c.Param("id")))
}

func (h *DocumentHandler) listGrants(c *gin.Context) {
	gs, err := h.uc.GrantLister.ListGrants(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(gs, grantView), err)
}

func (h *DocumentHandler) listShareCandidates(c *gin.Context) {
	ms, err := h.uc.CandidateLister.ListShareCandidates(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(ms, memberView), err)
}

func (h *DocumentHandler) share(c *gin.Context) {
	var body shareBody
	if !bind(c, &body) {
		return
	}
	g, err := h.uc.Sharer.Share(c, actor(c), tenant(c), c.Param("id"), body.input())
	respond(c, http.StatusCreated, grantView(g), err)
}

func (h *DocumentHandler) revoke(c *gin.Context) {
	var body shareBody
	if !bind(c, &body) {
		return
	}
	noContent(c, h.uc.ShareRevoker.Revoke(c, actor(c), tenant(c), c.Param("id"), body.input()))
}
