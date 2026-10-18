package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handlers) listMembers(c *gin.Context) {
	if !pathUUID(c, "id") {
		return
	}
	ms, err := h.app.ListMembers(c, actor(c), c.Param("id"), c.Query("affiliation"))
	respond(c, http.StatusOK, mapSlice(ms, memberView), err)
}

func (h *handlers) addMember(c *gin.Context) {
	var body struct {
		UserID      string `json:"user_id"`
		Affiliation string `json:"affiliation"`
		Role        string `json:"role"`
	}
	if !pathUUID(c, "id") || !bind(c, &body) {
		return
	}
	m, err := h.app.AddMember(c, actor(c), c.Param("id"), body.UserID, body.Affiliation, body.Role)
	respond(c, http.StatusCreated, memberView(m), err)
}

func (h *handlers) changeRole(c *gin.Context) {
	var body struct {
		Role string `json:"role"`
	}
	if !pathUUID(c, "id") || !pathUUID(c, "userId") || !bind(c, &body) {
		return
	}
	m, err := h.app.ChangeRole(c, actor(c), c.Param("id"), c.Param("userId"), body.Role)
	respond(c, http.StatusOK, memberView(m), err)
}

func (h *handlers) removeMember(c *gin.Context) {
	if !pathUUID(c, "id") || !pathUUID(c, "userId") {
		return
	}
	if err := h.app.RemoveMember(c, actor(c), c.Param("id"), c.Param("userId")); err != nil {
		writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
