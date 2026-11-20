package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/handler/http/mapper"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

// PhotoUsecases is everything the photo handler calls.
type PhotoUsecases struct {
	Lister          usecase.PhotoLister
	Getter          usecase.PhotoGetter
	Creator         usecase.PhotoCreator
	Renamer         usecase.PhotoRenamer
	Deleter         usecase.PhotoDeleter
	GrantLister     usecase.PhotoGrantLister
	CandidateLister usecase.PhotoShareCandidateLister
	Sharer          usecase.PhotoSharer
	ShareRevoker    usecase.PhotoShareRevoker
}

type PhotoHandler struct{ uc PhotoUsecases }

func NewPhotoHandler(uc PhotoUsecases) *PhotoHandler { return &PhotoHandler{uc: uc} }

func (h *PhotoHandler) Register(r Routes) {
	g := r.Tenanted
	g.GET("/photos", h.list)
	g.POST("/photos", h.create)
	g.GET("/photos/:id", h.get)
	g.PATCH("/photos/:id", h.rename)
	g.DELETE("/photos/:id", h.delete)
	g.GET("/photos/:id/grants", h.listGrants)
	g.GET("/photos/:id/share-candidates", h.listShareCandidates)
	g.POST("/photos/:id/grants", h.share)
	g.DELETE("/photos/:id/grants", h.revoke)
}

func (h *PhotoHandler) list(c *gin.Context) {
	ps, err := h.uc.Lister.List(c, input.PhotoLister{Actor: actor(c), TenantID: tenant(c), AlbumID: c.Query("album_id"), ProjectID: c.Query("project_id")})
	respond(c, http.StatusOK, mapSlice(ps.Items, mapper.VisiblePhotoView), err)
}

func (h *PhotoHandler) create(c *gin.Context) {
	var body map[string]any
	if !bind(c, &body) {
		return
	}
	in := input.PhotoCreator{Actor: actor(c), TenantID: tenant(c)}
	in.AlbumID, _ = body["album_id"].(string)
	in.Name, _ = body["name"].(string)
	in.MimeType, _ = body["mime_type"].(string)
	size, ok := body["byte_size"].(float64)
	if !ok || size != float64(int64(size)) {
		writeErr(c, model.Invalid("byte_size must be an integer"))
		return
	}
	in.ByteSize = int64(size)
	p, err := h.uc.Creator.Create(c, in)
	respond(c, http.StatusCreated, mapper.PhotoView(p.Photo), err)
}

func (h *PhotoHandler) get(c *gin.Context) {
	p, err := h.uc.Getter.Get(c, input.PhotoGetter{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id")})
	respond(c, http.StatusOK, mapper.VisiblePhotoView(p.Visible), err)
}

func (h *PhotoHandler) rename(c *gin.Context) {
	var body struct{ Name string }
	if !bind(c, &body) {
		return
	}
	p, err := h.uc.Renamer.Rename(c, input.PhotoRenamer{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id"), Name: body.Name})
	respond(c, http.StatusOK, mapper.PhotoView(p.Photo), err)
}

func (h *PhotoHandler) delete(c *gin.Context) {
	noContent(c, h.uc.Deleter.Delete(c, input.PhotoDeleter{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id")}))
}

func (h *PhotoHandler) listGrants(c *gin.Context) {
	gs, err := h.uc.GrantLister.ListGrants(c, input.PhotoGrantLister{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id")})
	respond(c, http.StatusOK, mapSlice(gs.Items, mapper.GrantView), err)
}

func (h *PhotoHandler) listShareCandidates(c *gin.Context) {
	ms, err := h.uc.CandidateLister.ListShareCandidates(c, input.PhotoShareCandidateLister{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id")})
	respond(c, http.StatusOK, mapSlice(ms.Items, mapper.MemberView), err)
}

func (h *PhotoHandler) share(c *gin.Context) {
	var body mapper.ShareBody
	if !bind(c, &body) {
		return
	}
	g, err := h.uc.Sharer.Share(c, input.PhotoSharer{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id"), Relation: body.Relation, Subject: body.Subject, UserID: body.UserID})
	respond(c, http.StatusCreated, mapper.GrantView(g.Grant), err)
}

func (h *PhotoHandler) revoke(c *gin.Context) {
	var body mapper.ShareBody
	if !bind(c, &body) {
		return
	}
	noContent(c, h.uc.ShareRevoker.Revoke(c, input.PhotoShareRevoker{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id"), Relation: body.Relation, Subject: body.Subject, UserID: body.UserID}))
}
