package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type ProjectRoleHandler struct {
	lister           usecase.ProjectRoleLister
	creator          usecase.ProjectRoleCreator
	permissionSetter usecase.ProjectRolePermissionSetter
	deleter          usecase.ProjectRoleDeleter
}

func NewProjectRoleHandler(lister usecase.ProjectRoleLister, creator usecase.ProjectRoleCreator, permissionSetter usecase.ProjectRolePermissionSetter, deleter usecase.ProjectRoleDeleter) *ProjectRoleHandler {
	return &ProjectRoleHandler{lister: lister, creator: creator, permissionSetter: permissionSetter, deleter: deleter}
}

func (h *ProjectRoleHandler) Register(r Routes) {
	g := r.Tenanted
	g.GET("/projects/:id/roles", h.list)
	g.POST("/projects/:id/roles", h.create)
	g.PUT("/projects/:id/roles/:roleId/permissions", h.setPermissions)
	g.DELETE("/projects/:id/roles/:roleId", h.delete)
}

func (h *ProjectRoleHandler) list(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	roles, err := h.lister.List(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(roles, projectRoleView), err)
}

func (h *ProjectRoleHandler) create(c *gin.Context) {
	var body struct {
		Name        string `json:"name"`
		Affiliation string `json:"affiliation"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	role, err := h.creator.Create(c, actor(c), tenant(c), c.Param("id"), body.Name, body.Affiliation)
	respond(c, http.StatusCreated, projectRoleView(role), err)
}

func (h *ProjectRoleHandler) setPermissions(c *gin.Context) {
	var body struct {
		Permissions []string `json:"permissions"`
	}
	if !pathUUID(c, "id") || !pathUUID(c, "roleId") || !bind(c, &body) {
		return
	}
	if body.Permissions == nil {
		writeErr(c, model.Invalid("permissions is required"))
		return
	}
	role, err := h.permissionSetter.SetPermissions(c, actor(c), tenant(c), c.Param("id"), c.Param("roleId"), body.Permissions)
	respond(c, http.StatusOK, projectRoleView(role), err)
}

func (h *ProjectRoleHandler) delete(c *gin.Context) {
	if !pathUUID(c, "id") || !pathUUID(c, "roleId") {
		return
	}
	noContent(c, h.deleter.Delete(c, actor(c), tenant(c), c.Param("id"), c.Param("roleId")))
}
