package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// AlbumUsecases is everything the album handler calls.
type AlbumUsecases struct {
	Lister           usecase.AlbumLister
	Getter           usecase.AlbumGetter
	Creator          usecase.AlbumCreator
	Renamer          usecase.AlbumRenamer
	VisibilitySetter usecase.AlbumVisibilitySetter
	Deleter          usecase.AlbumDeleter
	GrantLister      usecase.AlbumGrantLister
	CandidateLister  usecase.AlbumShareCandidateLister
	Sharer           usecase.AlbumSharer
	ShareRevoker     usecase.AlbumShareRevoker
}

type AlbumHandler struct{ uc AlbumUsecases }

func NewAlbumHandler(uc AlbumUsecases) *AlbumHandler { return &AlbumHandler{uc: uc} }

func (h *AlbumHandler) Register(r Routes) {
	g := r.Tenanted
	g.GET("/albums", h.list)
	g.POST("/albums", h.create)
	g.GET("/albums/:id", h.get)
	g.PATCH("/albums/:id", h.rename)
	g.DELETE("/albums/:id", h.delete)
	g.PUT("/albums/:id/visibility", h.setVisibility)
	g.GET("/albums/:id/grants", h.listGrants)
	g.GET("/albums/:id/share-candidates", h.listShareCandidates)
	g.POST("/albums/:id/grants", h.share)
	g.DELETE("/albums/:id/grants", h.revoke)
}

func albumView(a entity.Album) map[string]any {
	return map[string]any{
		"id": a.ID, "tenant_id": a.TenantID, "parent_id": a.ParentID, "name": a.Name, "is_private": a.IsPrivate,
		"created_by": a.CreatedBy, "created_at": a.CreatedAt,
	}
}

func visibleAlbumView(v entity.Visible[entity.Album]) map[string]any {
	m := albumView(v.Value)
	m["permissions"] = v.Perms
	return m
}

func (h *AlbumHandler) list(c *gin.Context) {
	as, err := h.uc.Lister.List(c, actor(c), tenant(c))
	respond(c, http.StatusOK, mapSlice(as, visibleAlbumView), err)
}

func (h *AlbumHandler) create(c *gin.Context) {
	var body struct {
		Name      string  `json:"name"`
		ParentID  *string `json:"parent_id"`
		IsPrivate *bool   `json:"is_private"`
	}
	if !bind(c, &body) {
		return
	}
	if body.IsPrivate == nil {
		writeErr(c, entity.Invalid("is_private is required"))
		return
	}
	a, err := h.uc.Creator.Create(c, actor(c), tenant(c), usecase.CreateAlbumInput{Name: body.Name, ParentID: body.ParentID, IsPrivate: *body.IsPrivate})
	respond(c, http.StatusCreated, albumView(a), err)
}

func (h *AlbumHandler) get(c *gin.Context) {
	a, err := h.uc.Getter.Get(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, visibleAlbumView(a), err)
}

func (h *AlbumHandler) rename(c *gin.Context) {
	var body struct{ Name string }
	if !bind(c, &body) {
		return
	}
	a, err := h.uc.Renamer.Rename(c, actor(c), tenant(c), c.Param("id"), body.Name)
	respond(c, http.StatusOK, albumView(a), err)
}

func (h *AlbumHandler) setVisibility(c *gin.Context) {
	var body visibilityBody
	if !bind(c, &body) {
		return
	}
	if body.IsPrivate == nil {
		writeErr(c, entity.Invalid("is_private is required"))
		return
	}
	a, err := h.uc.VisibilitySetter.SetVisibility(c, actor(c), tenant(c), c.Param("id"), *body.IsPrivate)
	respond(c, http.StatusOK, albumView(a), err)
}

func (h *AlbumHandler) delete(c *gin.Context) {
	noContent(c, h.uc.Deleter.Delete(c, actor(c), tenant(c), c.Param("id")))
}

func (h *AlbumHandler) listGrants(c *gin.Context) {
	gs, err := h.uc.GrantLister.ListGrants(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(gs, grantView), err)
}

func (h *AlbumHandler) listShareCandidates(c *gin.Context) {
	ms, err := h.uc.CandidateLister.ListShareCandidates(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(ms, memberView), err)
}

func (h *AlbumHandler) share(c *gin.Context) {
	var body shareBody
	if !bind(c, &body) {
		return
	}
	g, err := h.uc.Sharer.Share(c, actor(c), tenant(c), c.Param("id"), body.input())
	respond(c, http.StatusCreated, grantView(g), err)
}

func (h *AlbumHandler) revoke(c *gin.Context) {
	var body shareBody
	if !bind(c, &body) {
		return
	}
	noContent(c, h.uc.ShareRevoker.Revoke(c, actor(c), tenant(c), c.Param("id"), body.input()))
}
