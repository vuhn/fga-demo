package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/handler/http/mapper"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
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
	roles, err := h.lister.List(c, input.ProjectRoleLister{Actor: actor(c), TenantID: tenant(c), ProjectID: c.Param("id")})
	respond(c, http.StatusOK, mapSlice(roles.Items, mapper.ProjectRoleView), err)
}

func (h *ProjectRoleHandler) create(c *gin.Context) {
	var body struct {
		Name        string `json:"name"`
		Affiliation string `json:"affiliation"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	role, err := h.creator.Create(c, input.ProjectRoleCreator{Actor: actor(c), TenantID: tenant(c), ProjectID: c.Param("id"), Name: body.Name, Affiliation: body.Affiliation})
	respond(c, http.StatusCreated, mapper.ProjectRoleView(role.ProjectRole), err)
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
	role, err := h.permissionSetter.SetPermissions(c, input.ProjectRolePermissionSetter{Actor: actor(c), TenantID: tenant(c), ProjectID: c.Param("id"), RoleID: c.Param("roleId"), Permissions: body.Permissions})
	respond(c, http.StatusOK, mapper.ProjectRoleView(role.ProjectRole), err)
}

func (h *ProjectRoleHandler) delete(c *gin.Context) {
	if !pathUUID(c, "id") || !pathUUID(c, "roleId") {
		return
	}
	noContent(c, h.deleter.Delete(c, input.ProjectRoleDeleter{Actor: actor(c), TenantID: tenant(c), ProjectID: c.Param("id"), RoleID: c.Param("roleId")}))
}
