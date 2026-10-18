package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type PermissionHandler struct {
	reader usecase.PermissionReader
}

func NewPermissionHandler(reader usecase.PermissionReader) *PermissionHandler {
	return &PermissionHandler{reader: reader}
}

func (h *PermissionHandler) Register(r Routes) {
	r.Authed.GET("/me/permissions", h.mine)
}

// mine returns the caller's own permissions on the platform and, with
// ?tenant_id=, on that tenant. The UI maps them to menus and buttons.
func (h *PermissionHandler) mine(c *gin.Context) {
	p, err := h.reader.Mine(c, actor(c), c.Query("tenant_id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	body := gin.H{"platform": p.Platform, "tenant_id": nil, "tenant": nil}
	if p.TenantID != "" {
		body["tenant_id"], body["tenant"] = p.TenantID, p.Tenant
	}
	c.JSON(http.StatusOK, body)
}
