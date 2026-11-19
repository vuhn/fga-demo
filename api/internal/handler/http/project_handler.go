package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type ProjectHandler struct {
	lister  usecase.ProjectLister
	getter  usecase.ProjectGetter
	creator usecase.ProjectCreator
	renamer usecase.ProjectRenamer
}

func NewProjectHandler(lister usecase.ProjectLister, getter usecase.ProjectGetter, creator usecase.ProjectCreator, renamer usecase.ProjectRenamer) *ProjectHandler {
	return &ProjectHandler{lister: lister, getter: getter, creator: creator, renamer: renamer}
}

func (h *ProjectHandler) Register(r Routes) {
	g := r.Tenanted
	g.GET("/projects", h.list)
	g.POST("/projects", h.create)
	g.GET("/projects/:id", h.get)
	g.PATCH("/projects/:id", h.rename)
}

func (h *ProjectHandler) list(c *gin.Context) {
	ps, err := h.lister.List(c, actor(c), tenant(c))
	respond(c, http.StatusOK, mapSlice(ps, visibleProjectView), err)
}

func (h *ProjectHandler) create(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if !bind(c, &body) {
		return
	}
	p, err := h.creator.Create(c, actor(c), tenant(c), body.Name)
	respond(c, http.StatusCreated, projectView(p), err)
}

func (h *ProjectHandler) get(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	p, err := h.getter.Get(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, visibleProjectView(p), err)
}

func (h *ProjectHandler) rename(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	p, err := h.renamer.Rename(c, actor(c), tenant(c), c.Param("id"), body.Name)
	respond(c, http.StatusOK, projectView(p), err)
}
