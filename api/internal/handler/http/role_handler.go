package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/handler/http/mapper"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type RoleHandler struct {
	lister           usecase.RoleLister
	getter           usecase.RoleGetter
	creator          usecase.RoleCreator
	updater          usecase.RoleUpdater
	permissionSetter usecase.RolePermissionSetter
	deleter          usecase.RoleDeleter
}

func NewRoleHandler(lister usecase.RoleLister, getter usecase.RoleGetter, creator usecase.RoleCreator, updater usecase.RoleUpdater, permissionSetter usecase.RolePermissionSetter, deleter usecase.RoleDeleter) *RoleHandler {
	return &RoleHandler{lister: lister, getter: getter, creator: creator, updater: updater, permissionSetter: permissionSetter, deleter: deleter}
}

func (h *RoleHandler) Register(r Routes) {
	r.Authed.GET("/tenants/:id/roles", h.list)
	r.Authed.POST("/tenants/:id/roles", h.create)
	r.Authed.GET("/tenants/:id/roles/:roleId", h.get)
	r.Authed.PATCH("/tenants/:id/roles/:roleId", h.update)
	r.Authed.PUT("/tenants/:id/roles/:roleId/permissions", h.setPermissions)
	r.Authed.DELETE("/tenants/:id/roles/:roleId", h.delete)
}

func (h *RoleHandler) list(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	roles, err := h.lister.List(c, input.RoleLister{Actor: actor(c), TenantID: c.Param("id")})
	respond(c, http.StatusOK, mapper.RoleViews(roles.Items), err)
}

func (h *RoleHandler) get(c *gin.Context) {
	if !pathUUID(c, "id") || !pathUUID(c, "roleId") {
		return
	}
	role, err := h.getter.Get(c, input.RoleGetter{Actor: actor(c), TenantID: c.Param("id"), RoleID: c.Param("roleId")})
	respond(c, http.StatusOK, mapper.RoleView(role.Role), err)
}

func (h *RoleHandler) create(c *gin.Context) {
	var body struct {
		Name        string `json:"name"`
		Affiliation string `json:"affiliation"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	role, err := h.creator.Create(c, input.RoleCreator{Actor: actor(c), TenantID: c.Param("id"), Name: body.Name, Affiliation: body.Affiliation})
	respond(c, http.StatusCreated, mapper.RoleView(role.Role), err)
}

func (h *RoleHandler) update(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if !pathUUID(c, "id") || !pathUUID(c, "roleId") || !bind(c, &body) {
		return
	}
	role, err := h.updater.UpdateName(c, input.RoleUpdater{Actor: actor(c), TenantID: c.Param("id"), RoleID: c.Param("roleId"), Name: body.Name})
	respond(c, http.StatusOK, mapper.RoleView(role.Role), err)
}

func (h *RoleHandler) setPermissions(c *gin.Context) {
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
	role, err := h.permissionSetter.SetPermissions(c, input.RolePermissionSetter{Actor: actor(c), TenantID: c.Param("id"), RoleID: c.Param("roleId"), Permissions: body.Permissions})
	respond(c, http.StatusOK, mapper.RoleView(role.Role), err)
}

func (h *RoleHandler) delete(c *gin.Context) {
	if !pathUUID(c, "id") || !pathUUID(c, "roleId") {
		return
	}
	noContent(c, h.deleter.Delete(c, input.RoleDeleter{Actor: actor(c), TenantID: c.Param("id"), RoleID: c.Param("roleId")}))
}
