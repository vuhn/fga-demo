package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
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

type projectRoleJSON struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	ProjectID   string         `json:"project_id"`
	Name        string         `json:"name"`
	Affiliation string         `json:"affiliation"`
	Permissions []string       `json:"permissions"`
	Assignees   []assigneeJSON `json:"assignees"`
	CreatedAt   time.Time      `json:"created_at"`
}

func projectRoleView(role entity.ProjectRole) projectRoleJSON {
	perms := role.Permissions
	if perms == nil {
		perms = []string{}
	}
	assignees := make([]assigneeJSON, 0, len(role.Assignees))
	for _, a := range role.Assignees {
		assignees = append(assignees, assigneeJSON{a.UserID, a.Email, a.Name})
	}
	return projectRoleJSON{role.ID, role.TenantID, role.ProjectID, role.Name, role.Affiliation, perms, assignees, role.CreatedAt}
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
		writeErr(c, entity.Invalid("permissions is required"))
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
