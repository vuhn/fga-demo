package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// FolderUsecases is everything the folder handler calls.
type FolderUsecases struct {
	Lister           usecase.FolderLister
	Getter           usecase.FolderGetter
	Creator          usecase.FolderCreator
	Renamer          usecase.FolderRenamer
	VisibilitySetter usecase.FolderVisibilitySetter
	Deleter          usecase.FolderDeleter
	GrantLister      usecase.FolderGrantLister
	CandidateLister  usecase.FolderShareCandidateLister
	Sharer           usecase.FolderSharer
	ShareRevoker     usecase.FolderShareRevoker
}

type FolderHandler struct{ uc FolderUsecases }

func NewFolderHandler(uc FolderUsecases) *FolderHandler { return &FolderHandler{uc: uc} }

func (h *FolderHandler) Register(r Routes) {
	g := r.Tenanted
	g.GET("/folders", h.list)
	g.POST("/folders", h.create)
	g.GET("/folders/:id", h.get)
	g.PATCH("/folders/:id", h.rename)
	g.DELETE("/folders/:id", h.delete)
	g.PUT("/folders/:id/visibility", h.setVisibility)
	g.GET("/folders/:id/grants", h.listGrants)
	g.GET("/folders/:id/share-candidates", h.listShareCandidates)
	g.POST("/folders/:id/grants", h.share)
	g.DELETE("/folders/:id/grants", h.revoke)
}

func folderView(f entity.Folder) map[string]any {
	return map[string]any{
		"id": f.ID, "tenant_id": f.TenantID, "parent_id": f.ParentID, "name": f.Name, "is_private": f.IsPrivate,
		"created_by": f.CreatedBy, "created_at": f.CreatedAt,
	}
}

func visibleFolderView(v entity.Visible[entity.Folder]) map[string]any {
	m := folderView(v.Value)
	m["permissions"] = v.Perms
	return m
}

func (h *FolderHandler) list(c *gin.Context) {
	fs, err := h.uc.Lister.List(c, actor(c), tenant(c))
	respond(c, http.StatusOK, mapSlice(fs, visibleFolderView), err)
}

func (h *FolderHandler) create(c *gin.Context) {
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
	f, err := h.uc.Creator.Create(c, actor(c), tenant(c), usecase.CreateFolderInput{Name: body.Name, ParentID: body.ParentID, IsPrivate: *body.IsPrivate})
	respond(c, http.StatusCreated, folderView(f), err)
}

func (h *FolderHandler) get(c *gin.Context) {
	f, err := h.uc.Getter.Get(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, visibleFolderView(f), err)
}

func (h *FolderHandler) rename(c *gin.Context) {
	var body struct{ Name string }
	if !bind(c, &body) {
		return
	}
	f, err := h.uc.Renamer.Rename(c, actor(c), tenant(c), c.Param("id"), body.Name)
	respond(c, http.StatusOK, folderView(f), err)
}

func (h *FolderHandler) setVisibility(c *gin.Context) {
	var body visibilityBody
	if !bind(c, &body) {
		return
	}
	if body.IsPrivate == nil {
		writeErr(c, entity.Invalid("is_private is required"))
		return
	}
	f, err := h.uc.VisibilitySetter.SetVisibility(c, actor(c), tenant(c), c.Param("id"), *body.IsPrivate)
	respond(c, http.StatusOK, folderView(f), err)
}

func (h *FolderHandler) delete(c *gin.Context) {
	noContent(c, h.uc.Deleter.Delete(c, actor(c), tenant(c), c.Param("id")))
}

func (h *FolderHandler) listGrants(c *gin.Context) {
	gs, err := h.uc.GrantLister.ListGrants(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(gs, grantView), err)
}

func (h *FolderHandler) listShareCandidates(c *gin.Context) {
	ms, err := h.uc.CandidateLister.ListShareCandidates(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(ms, memberView), err)
}

func (h *FolderHandler) share(c *gin.Context) {
	var body shareBody
	if !bind(c, &body) {
		return
	}
	g, err := h.uc.Sharer.Share(c, actor(c), tenant(c), c.Param("id"), body.input())
	respond(c, http.StatusCreated, grantView(g), err)
}

func (h *FolderHandler) revoke(c *gin.Context) {
	var body shareBody
	if !bind(c, &body) {
		return
	}
	noContent(c, h.uc.ShareRevoker.Revoke(c, actor(c), tenant(c), c.Param("id"), body.input()))
}
