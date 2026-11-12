package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type ProjectMemberHandler struct {
	lister      usecase.ProjectMemberLister
	candidates  usecase.ProjectMemberCandidateLister
	adder       usecase.ProjectMemberAdder
	roleChanger usecase.ProjectMemberRoleChanger
	remover     usecase.ProjectMemberRemover
}

func NewProjectMemberHandler(lister usecase.ProjectMemberLister, candidates usecase.ProjectMemberCandidateLister, adder usecase.ProjectMemberAdder, roleChanger usecase.ProjectMemberRoleChanger, remover usecase.ProjectMemberRemover) *ProjectMemberHandler {
	return &ProjectMemberHandler{lister: lister, candidates: candidates, adder: adder, roleChanger: roleChanger, remover: remover}
}

func (h *ProjectMemberHandler) Register(r Routes) {
	g := r.Tenanted
	g.GET("/projects/:id/members", h.list)
	g.GET("/projects/:id/member-candidates", h.listCandidates)
	g.POST("/projects/:id/members", h.add)
	g.PATCH("/projects/:id/members/:userId", h.changeRole)
	g.DELETE("/projects/:id/members/:userId", h.remove)
}

type projectMemberJSON struct {
	UserID      string         `json:"user_id"`
	Email       string         `json:"email"`
	Name        string         `json:"name"`
	Affiliation string         `json:"affiliation"`
	Role        string         `json:"role"`
	RoleName    string         `json:"role_name,omitempty"`
	Company     companyRefJSON `json:"company"`
}

func projectMemberView(m entity.ProjectMember) projectMemberJSON {
	return projectMemberJSON{m.UserID, m.Email, m.Name, m.Affiliation, m.Role, m.RoleName, companyRefJSON{m.CompanyID, m.CompanyName}}
}

func (h *ProjectMemberHandler) list(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	ms, err := h.lister.List(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(ms, projectMemberView), err)
}

func (h *ProjectMemberHandler) listCandidates(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	ms, err := h.candidates.ListCandidates(c, actor(c), tenant(c), c.Param("id"))
	respond(c, http.StatusOK, mapSlice(ms, memberView), err)
}

func (h *ProjectMemberHandler) add(c *gin.Context) {
	var body struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	m, err := h.adder.Add(c, actor(c), tenant(c), c.Param("id"), usecase.AddProjectMemberInput{UserID: body.UserID, Role: body.Role})
	respond(c, http.StatusCreated, projectMemberView(m), err)
}

func (h *ProjectMemberHandler) changeRole(c *gin.Context) {
	var body struct {
		Role string `json:"role"`
	}
	if !pathUUID(c, "id") || !pathUUID(c, "userId") || !bind(c, &body) {
		return
	}
	m, err := h.roleChanger.ChangeRole(c, actor(c), tenant(c), c.Param("id"), c.Param("userId"), body.Role)
	respond(c, http.StatusOK, projectMemberView(m), err)
}

func (h *ProjectMemberHandler) remove(c *gin.Context) {
	if !pathUUID(c, "id") || !pathUUID(c, "userId") {
		return
	}
	noContent(c, h.remover.Remove(c, actor(c), tenant(c), c.Param("id"), c.Param("userId")))
}
