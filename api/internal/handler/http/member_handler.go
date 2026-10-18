package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type MemberHandler struct {
	lister      usecase.MemberLister
	adder       usecase.MemberAdder
	roleChanger usecase.MemberRoleChanger
	remover     usecase.MemberRemover
}

func NewMemberHandler(lister usecase.MemberLister, adder usecase.MemberAdder, roleChanger usecase.MemberRoleChanger, remover usecase.MemberRemover) *MemberHandler {
	return &MemberHandler{lister: lister, adder: adder, roleChanger: roleChanger, remover: remover}
}

func (h *MemberHandler) Register(r Routes) {
	r.Authed.GET("/tenants/:id/members", h.list)
	r.Authed.POST("/tenants/:id/members", h.add)
	r.Authed.PATCH("/tenants/:id/members/:userId", h.changeRole)
	r.Authed.DELETE("/tenants/:id/members/:userId", h.remove)
}

func (h *MemberHandler) list(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	ms, err := h.lister.List(c, actor(c), c.Param("id"), c.Query("affiliation"))
	respond(c, http.StatusOK, mapSlice(ms, memberView), err)
}

func (h *MemberHandler) add(c *gin.Context) {
	var body struct {
		UserID      string `json:"user_id"`
		Affiliation string `json:"affiliation"`
		Role        string `json:"role"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	m, err := h.adder.Add(c, actor(c), c.Param("id"), usecase.AddMemberInput{UserID: body.UserID, Affiliation: body.Affiliation, Role: body.Role})
	respond(c, http.StatusCreated, memberView(m), err)
}

func (h *MemberHandler) changeRole(c *gin.Context) {
	var body struct {
		Role string `json:"role"`
	}
	if !pathUUID(c, "id") || !pathUUID(c, "userId") || !bind(c, &body) {
		return
	}
	m, err := h.roleChanger.ChangeRole(c, actor(c), c.Param("id"), c.Param("userId"), body.Role)
	respond(c, http.StatusOK, memberView(m), err)
}

func (h *MemberHandler) remove(c *gin.Context) {
	if !pathUUID(c, "id") || !pathUUID(c, "userId") {
		return
	}
	noContent(c, h.remover.Remove(c, actor(c), c.Param("id"), c.Param("userId")))
}
