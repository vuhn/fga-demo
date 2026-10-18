package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
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

func photoView(p entity.Photo) map[string]any {
	return map[string]any{
		"id": p.ID, "tenant_id": p.TenantID, "album_id": p.AlbumID, "name": p.Name,
		"mime_type": p.MimeType, "byte_size": p.ByteSize, "created_by": p.CreatedBy, "created_at": p.CreatedAt,
	}
}

func visiblePhotoView(v entity.Visible[entity.Photo]) map[string]any {
	m := photoView(v.Value)
	m["permissions"] = v.Perms
	return m
}

func (h *PhotoHandler) list(c *gin.Context) {
	ps, err := h.uc.Lister.List(c, actor(c), tenant(c), c.Query("album_id"))
	respond(c, http.StatusOK, mapSlice(ps, visiblePhotoView), err)
}

func (h *PhotoHandler) create(c *gin.Context) {
	var body map[string]any
	if !bind(c, &body) {
		return
	}
	in := usecase.CreatePhotoInput{}
	in.AlbumID, _ = body["album_id"].(string)
	in.Name, _ = body["name"].(string)
	in.MimeType, _ = body["mime_type"].(string)
	size, ok := body["byte_size"].(float64)
	if !ok || size != float64(int64(size)) {
		writeErr(c, entity.Invalid("byte_size must be an integer"))
		return
	}
	in.ByteSize = int64(size)
	p, err := h.uc.Creator.Create(c, actor(c), tenant(c), in)
	respond(c, http.StatusCreated, photoView(p), err)
}

func (h *PhotoHandler) get(c *gin.Context) {
	p, err := h.uc.Getter.Get(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, visiblePhotoView(p), err)
}

func (h *PhotoHandler) rename(c *gin.Context) {
	var body struct{ Name string }
	if !bind(c, &body) {
		return
	}
	p, err := h.uc.Renamer.Rename(c, actor(c), tenant(c), c.Param("id"), body.Name)
	respond(c, http.StatusOK, photoView(p), err)
}

func (h *PhotoHandler) delete(c *gin.Context) {
	noContent(c, h.uc.Deleter.Delete(c, actor(c), tenant(c), c.Param("id")))
}

func (h *PhotoHandler) listGrants(c *gin.Context) {
	gs, err := h.uc.GrantLister.ListGrants(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(gs, grantView), err)
}

func (h *PhotoHandler) listShareCandidates(c *gin.Context) {
	ms, err := h.uc.CandidateLister.ListShareCandidates(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(ms, memberView), err)
}

func (h *PhotoHandler) share(c *gin.Context) {
	var body shareBody
	if !bind(c, &body) {
		return
	}
	g, err := h.uc.Sharer.Share(c, actor(c), tenant(c), c.Param("id"), body.input())
	respond(c, http.StatusCreated, grantView(g), err)
}

func (h *PhotoHandler) revoke(c *gin.Context) {
	var body shareBody
	if !bind(c, &body) {
		return
	}
	noContent(c, h.uc.ShareRevoker.Revoke(c, actor(c), tenant(c), c.Param("id"), body.input()))
}
