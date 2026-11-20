package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/handler/http/mapper"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
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

func (h *DocumentHandler) list(c *gin.Context) {
	ds, err := h.uc.Lister.List(c, input.DocumentLister{Actor: actor(c), TenantID: tenant(c), FolderID: c.Query("folder_id"), ProjectID: c.Query("project_id")})
	respond(c, http.StatusOK, mapSlice(ds.Items, mapper.VisibleDocumentView), err)
}

func (h *DocumentHandler) create(c *gin.Context) {
	var body map[string]any
	if !bind(c, &body) {
		return
	}
	in := input.DocumentCreator{Actor: actor(c), TenantID: tenant(c)}
	in.FolderID, _ = body["folder_id"].(string)
	in.Name, _ = body["name"].(string)
	in.MimeType, _ = body["mime_type"].(string)
	size, ok := body["byte_size"].(float64)
	if !ok || size != float64(int64(size)) {
		writeErr(c, model.Invalid("byte_size must be an integer"))
		return
	}
	in.ByteSize = int64(size)
	d, err := h.uc.Creator.Create(c, in)
	respond(c, http.StatusCreated, mapper.DocumentView(d.Document), err)
}

func (h *DocumentHandler) get(c *gin.Context) {
	d, err := h.uc.Getter.Get(c, input.DocumentGetter{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id")})
	respond(c, http.StatusOK, mapper.VisibleDocumentView(d.Visible), err)
}

func (h *DocumentHandler) rename(c *gin.Context) {
	var body struct{ Name string }
	if !bind(c, &body) {
		return
	}
	d, err := h.uc.Renamer.Rename(c, input.DocumentRenamer{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id"), Name: body.Name})
	respond(c, http.StatusOK, mapper.DocumentView(d.Document), err)
}

func (h *DocumentHandler) delete(c *gin.Context) {
	noContent(c, h.uc.Deleter.Delete(c, input.DocumentDeleter{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id")}))
}

func (h *DocumentHandler) listGrants(c *gin.Context) {
	gs, err := h.uc.GrantLister.ListGrants(c, input.DocumentGrantLister{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id")})
	respond(c, http.StatusOK, mapSlice(gs.Items, mapper.GrantView), err)
}

func (h *DocumentHandler) listShareCandidates(c *gin.Context) {
	ms, err := h.uc.CandidateLister.ListShareCandidates(c, input.DocumentShareCandidateLister{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id")})
	respond(c, http.StatusOK, mapSlice(ms.Items, mapper.MemberView), err)
}

func (h *DocumentHandler) share(c *gin.Context) {
	var body mapper.ShareBody
	if !bind(c, &body) {
		return
	}
	g, err := h.uc.Sharer.Share(c, input.DocumentSharer{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id"), Relation: body.Relation, Subject: body.Subject, UserID: body.UserID})
	respond(c, http.StatusCreated, mapper.GrantView(g.Grant), err)
}

func (h *DocumentHandler) revoke(c *gin.Context) {
	var body mapper.ShareBody
	if !bind(c, &body) {
		return
	}
	noContent(c, h.uc.ShareRevoker.Revoke(c, input.DocumentShareRevoker{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id"), Relation: body.Relation, Subject: body.Subject, UserID: body.UserID}))
}
