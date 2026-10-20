package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
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

type roleJSON struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	Name        string         `json:"name"`
	Affiliation string         `json:"affiliation"`
	Permissions []string       `json:"permissions"`
	Assignees   []assigneeJSON `json:"assignees"`
	CreatedAt   time.Time      `json:"created_at"`
}

type assigneeJSON struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

func roleView(role entity.Role) roleJSON {
	perms := role.Permissions
	if perms == nil {
		perms = []string{}
	}
	assignees := make([]assigneeJSON, 0, len(role.Assignees))
	for _, a := range role.Assignees {
		assignees = append(assignees, assigneeJSON{a.UserID, a.Email, a.Name})
	}
	return roleJSON{role.ID, role.TenantID, role.Name, role.Affiliation, perms, assignees, role.CreatedAt}
}

func roleViews(roles []entity.Role) []roleJSON {
	out := make([]roleJSON, 0, len(roles))
	for _, role := range roles {
		out = append(out, roleView(role))
	}
	return out
}

func (h *RoleHandler) list(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	roles, err := h.lister.List(c, actor(c), c.Param("id"))
	respond(c, http.StatusOK, roleViews(roles), err)
}

func (h *RoleHandler) get(c *gin.Context) {
	if !pathUUID(c, "id") || !pathUUID(c, "roleId") {
		return
	}
	role, err := h.getter.Get(c, actor(c), c.Param("id"), c.Param("roleId"))
	respond(c, http.StatusOK, roleView(role), err)
}

func (h *RoleHandler) create(c *gin.Context) {
	var body struct {
		Name        string `json:"name"`
		Affiliation string `json:"affiliation"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	role, err := h.creator.Create(c, actor(c), c.Param("id"), body.Name, body.Affiliation)
	respond(c, http.StatusCreated, roleView(role), err)
}

func (h *RoleHandler) update(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if !pathUUID(c, "id") || !pathUUID(c, "roleId") || !bind(c, &body) {
		return
	}
	role, err := h.updater.UpdateName(c, actor(c), c.Param("id"), c.Param("roleId"), body.Name)
	respond(c, http.StatusOK, roleView(role), err)
}

func (h *RoleHandler) setPermissions(c *gin.Context) {
	var body struct {
		Permissions []string `json:"permissions"`
	}
	if !pathUUID(c, "id") || !pathUUID(c, "roleId") || !bind(c, &body) {
		return
	}
	if body.Permissions == nil {
		writeErr(c, entity.Invalid("permissions is required"))
		return
	}
	role, err := h.permissionSetter.SetPermissions(c, actor(c), c.Param("id"), c.Param("roleId"), body.Permissions)
	respond(c, http.StatusOK, roleView(role), err)
}

func (h *RoleHandler) delete(c *gin.Context) {
	if !pathUUID(c, "id") || !pathUUID(c, "roleId") {
		return
	}
	noContent(c, h.deleter.Delete(c, actor(c), c.Param("id"), c.Param("roleId")))
}
