package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/handler/http/mapper"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
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

func (h *FolderHandler) list(c *gin.Context) {
	fs, err := h.uc.Lister.List(c, input.FolderLister{Actor: actor(c), TenantID: tenant(c), ProjectID: c.Query("project_id")})
	respond(c, http.StatusOK, mapSlice(fs.Items, mapper.VisibleFolderView), err)
}

func (h *FolderHandler) create(c *gin.Context) {
	var body struct {
		ProjectID string  `json:"project_id"`
		Name      string  `json:"name"`
		ParentID  *string `json:"parent_id"`
		IsPrivate *bool   `json:"is_private"`
	}
	if !bind(c, &body) {
		return
	}
	if body.IsPrivate == nil {
		writeErr(c, model.Invalid("is_private is required"))
		return
	}
	f, err := h.uc.Creator.Create(c, input.FolderCreator{Actor: actor(c), TenantID: tenant(c), ProjectID: body.ProjectID, Name: body.Name, ParentID: body.ParentID, IsPrivate: *body.IsPrivate})
	respond(c, http.StatusCreated, mapper.FolderView(f.Folder), err)
}

func (h *FolderHandler) get(c *gin.Context) {
	f, err := h.uc.Getter.Get(c, input.FolderGetter{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id")})
	respond(c, http.StatusOK, mapper.VisibleFolderView(f.Visible), err)
}

func (h *FolderHandler) rename(c *gin.Context) {
	var body struct{ Name string }
	if !bind(c, &body) {
		return
	}
	f, err := h.uc.Renamer.Rename(c, input.FolderRenamer{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id"), Name: body.Name})
	respond(c, http.StatusOK, mapper.FolderView(f.Folder), err)
}

func (h *FolderHandler) setVisibility(c *gin.Context) {
	var body mapper.VisibilityBody
	if !bind(c, &body) {
		return
	}
	if body.IsPrivate == nil {
		writeErr(c, model.Invalid("is_private is required"))
		return
	}
	f, err := h.uc.VisibilitySetter.SetVisibility(c, input.FolderVisibilitySetter{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id"), IsPrivate: *body.IsPrivate})
	respond(c, http.StatusOK, mapper.FolderView(f.Folder), err)
}

func (h *FolderHandler) delete(c *gin.Context) {
	noContent(c, h.uc.Deleter.Delete(c, input.FolderDeleter{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id")}))
}

func (h *FolderHandler) listGrants(c *gin.Context) {
	gs, err := h.uc.GrantLister.ListGrants(c, input.FolderGrantLister{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id")})
	respond(c, http.StatusOK, mapSlice(gs.Items, mapper.GrantView), err)
}

func (h *FolderHandler) listShareCandidates(c *gin.Context) {
	ms, err := h.uc.CandidateLister.ListShareCandidates(c, input.FolderShareCandidateLister{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id")})
	respond(c, http.StatusOK, mapSlice(ms.Items, mapper.MemberView), err)
}

func (h *FolderHandler) share(c *gin.Context) {
	var body mapper.ShareBody
	if !bind(c, &body) {
		return
	}
	g, err := h.uc.Sharer.Share(c, input.FolderSharer{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id"), Relation: body.Relation, Subject: body.Subject, UserID: body.UserID})
	respond(c, http.StatusCreated, mapper.GrantView(g.Grant), err)
}

func (h *FolderHandler) revoke(c *gin.Context) {
	var body mapper.ShareBody
	if !bind(c, &body) {
		return
	}
	noContent(c, h.uc.ShareRevoker.Revoke(c, input.FolderShareRevoker{Actor: actor(c), TenantID: tenant(c), ID: c.Param("id"), Relation: body.Relation, Subject: body.Subject, UserID: body.UserID}))
}
